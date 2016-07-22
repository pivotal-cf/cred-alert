package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/jessevdk/go-flags"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/db"
	"cred-alert/ingestor"
	"cred-alert/metrics"
	"cred-alert/queue"
)

type Opts struct {
	Port      uint16   `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`
	Whitelist []string `short:"i" long:"ignore-repos" description:"comma separated list of repo names to ignore. The names may be regex patterns." env:"IGNORED_REPOS" env-delim:"," value-name:"REPO_LIST"`

	GitHub struct {
		WebhookToken string `short:"w" long:"webhook-token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_KEY" value-name:"TOKEN" required:"true"`
	} `group:"GitHub Options"`

	Datadog struct {
		APIKey      string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATA_DOG_API_KEY" value-name:"KEY"`
		Environment string `long:"datadog-environment" description:"environment tag for datadog" env:"DATA_DOG_ENVIRONMENT_TAG" value-name:"NAME" default:"development"`
	} `group:"Datadog Options"`

	AWS struct {
		AwsAccessKey       string `long:"aws-access-key" description:"access key for aws SQS service" env:"AWS_ACCESS_KEY" value-name:"ACCESS_KEY"`
		AwsSecretAccessKey string `long:"aws-secret-key" description:"secret access key for aws SQS service" env:"AWS_SECRET_ACCESS_KEY" value-name:"SECRET_KEY"`
		AwsRegion          string `long:"aws-region" description:"aws region for SQS service" env:"AWS_REGION" value-name:"REGION"`
		SqsQueueName       string `long:"sqs-queue-name" description:"queue name to use with SQS" env:"SQS_QUEUE_NAME" value-name:"QUEUE_NAME"`
	} `group:"AWS Options"`

	MySQL struct {
		Username string `long:"mysql-username" description:"MySQL username" value-name:"USERNAME"`
		Password string `long:"mysql-password" description:"MySQL password" value-name:"PASSWORD"`
		Hostname string `long:"mysql-hostname" description:"MySQL hostname" value-name:"HOSTNAME"`
		Port     uint16 `long:"mysql-port" description:"MySQL port" value-name:"PORT"`
		DBName   string `long:"mysql-dbname" description:"MySQL database name" value-name:"DBNAME"`
	}
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("cred-alert")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	taskQueue, err := createQueue(opts, logger)
	if err != nil {
		logger.Error("Could not create queue", err)
		os.Exit(1)
	}

	emitter := metrics.BuildEmitter(opts.Datadog.APIKey, opts.Datadog.Environment)
	repoWhitelist := ingestor.BuildWhitelist(opts.Whitelist...)
	generator := queue.NewGenerator()

	database, err := createDB(logger, opts)
	if err != nil {
		logger.Error("Fatal Error: Could not connect to db", err)
		os.Exit(1)
	}
	defer database.Close()
	commitRepository := db.NewCommitRepository(database)

	in := ingestor.NewIngestor(taskQueue, emitter, repoWhitelist, generator, commitRepository)

	router := http.NewServeMux()
	router.Handle("/webhook", ingestor.Handler(logger, in, opts.GitHub.WebhookToken))

	members := []grouper.Member{
		{"api", http_server.New(
			fmt.Sprintf(":%d", opts.Port),
			router,
		)},
		{"debug", http_server.New(
			"127.0.0.1:6060",
			debugHandler(),
		)},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	logger.Info("starting-server", lager.Data{
		"port": opts.Port,
	})

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("running-server-failed", err)
	}
}

func createQueue(opts Opts, logger lager.Logger) (queue.Queue, error) {
	if sqsValuesExist(opts) {
		return createSqsQueue(opts)
	}

	return queue.NewNullQueue(logger), nil
}

func sqsValuesExist(opts Opts) bool {
	if opts.AWS.AwsAccessKey != "" &&
		opts.AWS.AwsSecretAccessKey != "" &&
		opts.AWS.AwsRegion != "" &&
		opts.AWS.SqsQueueName != "" {

		return true
	}

	return false
}

func createSqsQueue(opts Opts) (queue.Queue, error) {
	creds := credentials.NewStaticCredentials(
		opts.AWS.AwsAccessKey,
		opts.AWS.AwsSecretAccessKey,
		"")
	config := aws.NewConfig().WithCredentials(creds).WithRegion(opts.AWS.AwsRegion)
	service := sqs.New(session.New(config))
	queue, err := queue.BuildSQSQueue(service, opts.AWS.SqsQueueName)
	if err != nil {
		return nil, err
	}
	return queue, nil
}

func debugHandler() http.Handler {
	debugRouter := http.NewServeMux()
	debugRouter.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	debugRouter.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	debugRouter.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	debugRouter.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	debugRouter.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	return debugRouter
}

func createDB(logger lager.Logger, opts Opts) (*gorm.DB, error) {
	var uri string
	var err error
	if os.Getenv("VCAP_SERVICES") != "" {
		uri, err = createDbUriFromVCAP(logger)
	} else {
		uri, err = createDbUriFromOpts(logger, opts)
	}

	if err != nil {
		logger.Error("Error getting db uri string: ", err)
		return nil, err
	}

	return gorm.Open("mysql", uri)
}

func createDbUriFromVCAP(logger lager.Logger) (string, error) {
	logger = logger.Session("creating-db-from-vcap")

	appEnv, err := cfenv.Current()
	if err != nil {
		logger.Error("Error getting CF environment", err)
		return "", err
	}

	service, err := appEnv.Services.WithName("cred-alert-mysql")
	if err != nil {
		logger.Error("Error getting cred-alert-mysql instance", err)
	}

	username, ok := service.Credentials["username"].(string)
	if !ok {
		return "", errors.New("Could not read username")
	}
	password, ok := service.Credentials["password"].(string)
	if !ok {
		return "", errors.New("Could not read password")
	}
	hostname, ok := service.Credentials["hostname"].(string)
	if !ok {
		return "", errors.New("Could not read hostname")
	}
	portF, ok := service.Credentials["port"].(float64)
	if !ok {
		return "", errors.New("Could not read port")
	}
	port := int(portF)
	name := service.Credentials["name"]

	if len(username) == 0 || len(password) == 0 {
		err := errors.New("Empty mysql username or password")
		logger.Error("MySQL parameters are incorrect", err)
		return "", err
	}

	uri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True",
		username, password, hostname, port, name)

	logger.Info("vcap-services.success")
	return uri, nil
}

func createDbUriFromOpts(logger lager.Logger, opts Opts) (string, error) {
	logger = logger.Session("creating-db-from-opts")

	uri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True",
		opts.MySQL.Username, opts.MySQL.Password, opts.MySQL.Hostname, opts.MySQL.Port, opts.MySQL.DBName)
	logger.Info("command-line.success")
	return uri, nil
}

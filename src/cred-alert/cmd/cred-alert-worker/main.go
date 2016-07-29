package main

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"golang.org/x/oauth2"

	_ "github.com/jinzhu/gorm/dialects/mysql"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/contraband/zest"
	"github.com/jessevdk/go-flags"
	"github.com/jinzhu/gorm"
	"github.com/rakyll/magicmime"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/githubclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/queue"
	"cred-alert/sniff"
	"cred-alert/worker"
)

type AWSOpts struct {
	AwsAccessKey       string `long:"aws-access-key" description:"access key for aws SQS service" env:"AWS_ACCESS_KEY" value-name:"ACCESS_KEY"`
	AwsSecretAccessKey string `long:"aws-secret-key" description:"secret access key for aws SQS service" env:"AWS_SECRET_ACCESS_KEY" value-name:"SECRET_KEY"`
	AwsRegion          string `long:"aws-region" description:"aws region for SQS service" env:"AWS_REGION" value-name:"REGION"`
	SqsQueueName       string `long:"sqs-queue-name" description:"queue name to use with SQS" env:"SQS_QUEUE_NAME" value-name:"QUEUE_NAME"`
}

type Opts struct {
	GitHub struct {
		AccessToken string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
	} `group:"GitHub Options"`

	Metrics struct {
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		YellerAPIKey  string `long:"yeller-api-key" description:"key to emit to yeller" env:"YELLER_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`

	Slack struct {
		WebhookUrl string `long:"slack-webhook-url" description:"Slack webhook URL" env:"SLACK_WEBHOOK_URL" value-name:"WEBHOOK"`
	} `group:"Slack Options"`

	AWS AWSOpts `group:"AWS Options"`

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

	logger := lager.NewLogger("cred-alert-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	if opts.Metrics.YellerAPIKey != "" {
		sink := zest.NewYellerSink(opts.Metrics.YellerAPIKey, opts.Metrics.Environment)
		logger.RegisterSink(sink)
	}

	logger.Info("starting")

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.GitHub.AccessToken},
	)
	httpClient := &http.Client{
		Timeout: time.Second,
		Transport: &oauth2.Transport{
			Source: tokenSource,
			Base: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	client := githubclient.NewClient(githubclient.DefaultGitHubURL, httpClient, emitter)
	notifier := notifications.NewSlackNotifier(opts.Slack.WebhookUrl)
	sniffer := sniff.NewDefaultSniffer()

	database, err := createDB(logger, opts)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	defer database.Close()

	diffScanRepository := db.NewDiffScanRepository(database)
	commitRepository := db.NewCommitRepository(database)

	taskQueue, err := createQueue(opts, logger)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	mimetype, err := magicmime.NewDecoder(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR)

	foreman := queue.NewForeman(
		client,
		sniffer,
		emitter,
		notifier,
		diffScanRepository,
		commitRepository,
		taskQueue,
		mimetype,
	)

	backgroundWorker := worker.New(logger, foreman, taskQueue, emitter)

	members := []grouper.Member{
		{"worker", backgroundWorker},
		{"debug", http_server.New(
			"127.0.0.1:6060",
			debugHandler(),
		)},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("running-server-failed", err)
	}
}

func createQueue(opts Opts, logger lager.Logger) (queue.Queue, error) {
	logger = logger.Session("create-queue")
	logger.Info("starting")

	if sqsValuesExist(opts.AWS) {
		logger.Debug("done")
		return createSqsQueue(logger, opts.AWS)
	}

	logger.Debug("done")
	return queue.NewNullQueue(logger), nil
}

func sqsValuesExist(awsOpts AWSOpts) bool {
	if awsOpts.AwsAccessKey != "" &&
		awsOpts.AwsSecretAccessKey != "" &&
		awsOpts.AwsRegion != "" &&
		awsOpts.SqsQueueName != "" {

		return true
	}

	return false
}

func createSqsQueue(logger lager.Logger, awsOpts AWSOpts) (queue.Queue, error) {
	logger = logger.Session("create-sqs-queue")

	creds := credentials.NewStaticCredentials(awsOpts.AwsAccessKey, awsOpts.AwsSecretAccessKey, "")
	config := aws.NewConfig().WithCredentials(creds).WithRegion(awsOpts.AwsRegion)
	service := sqs.New(session.New(config))

	queue, err := queue.BuildSQSQueue(service, awsOpts.SqsQueueName)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	logger.Debug("done")
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
	logger = logger.Session("create-db")
	logger.Info("starting")

	var uri string
	if os.Getenv("VCAP_SERVICES") != "" {
		var err error
		uri, err = createDbUriFromVCAP(logger)
		if err != nil {
			logger.Error("failed", err)
			return nil, err
		}
	} else {
		uri = db.NewDSN(opts.MySQL.Username, opts.MySQL.Password, opts.MySQL.DBName, opts.MySQL.Hostname, int(opts.MySQL.Port))
	}

	logger.Debug("done")
	return migrations.LockDBAndMigrate(logger, "mysql", uri)
}

func createDbUriFromVCAP(logger lager.Logger) (string, error) {
	logger = logger.Session("creating-db-from-vcap")

	appEnv, err := cfenv.Current()
	if err != nil {
		logger.Error("failed", err)
		return "", err
	}

	service, err := appEnv.Services.WithName("cred-alert-mysql")
	if err != nil {
		logger.Error("Error getting cred-alert-mysql instance", err)
	}

	username, ok := service.Credentials["username"].(string)
	if !ok {
		err = errors.New("Could not read username")
		logger.Error("failed", err)
		return "", err
	}
	password, ok := service.Credentials["password"].(string)
	if !ok {
		err = errors.New("Could not read password")
		logger.Error("failed", err)
		return "", err
	}
	hostname, ok := service.Credentials["hostname"].(string)
	if !ok {
		err = errors.New("Could not read hostname")
		logger.Error("failed", err)
		return "", err
	}
	portF, ok := service.Credentials["port"].(float64)
	if !ok {
		err = errors.New("Could not read port")
		logger.Error("failed", err)
		return "", err
	}
	port := int(portF)
	name := service.Credentials["name"]

	if len(username) == 0 || len(password) == 0 {
		err = errors.New("Empty mysql username or password")
		logger.Error("failed", err)
		return "", err
	}

	database, ok := name.(string)
	if !ok {
		err = errors.New("non-string database name given")
		logger.Error("failed", err)
		return "", err
	}

	logger.Debug("done")
	return db.NewDSN(username, password, database, hostname, port), nil
}

package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/jessevdk/go-flags"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

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

	logger := lager.NewLogger("cred-alert-ingestor")
	logger.Info("started")

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		logger.Error("failed", err)
		os.Exit(1)
	}

	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	taskQueue, err := createQueue(opts, logger)
	if err != nil {
		logger.Error("failed", err)
		os.Exit(1)
	}

	emitter := metrics.BuildEmitter(opts.Datadog.APIKey, opts.Datadog.Environment)
	repoWhitelist := ingestor.BuildWhitelist(opts.Whitelist...)
	generator := queue.NewGenerator()

	in := ingestor.NewIngestor(taskQueue, emitter, repoWhitelist, generator)

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
		logger.Session("starting-server").Error("failed", err)
	}
}

func createQueue(opts Opts, logger lager.Logger) (queue.Queue, error) {
	logger = logger.Session("creating-queue")

	if sqsValuesExist(opts) {
		logger.Session("sqs-queue").Info("done")
		return createSqsQueue(opts)
	}

	logger.Session("null-queue").Info("done")
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

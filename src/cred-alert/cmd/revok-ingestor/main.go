package main

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/net/context"

	"cred-alert/crypto"
	"cred-alert/ingestor"
	"cred-alert/metrics"
	"cred-alert/queue"
	"cred-alert/revok"
)

type Opts struct {
	Port uint16 `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`

	GitHub struct {
		WebhookSecretTokens []string `short:"w" long:"github-webhook-secret-token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_TOKENS" env-delim:"," value-name:"TOKENS" required:"true"`
	} `group:"GitHub Options"`

	PubSub struct {
		ProjectName string `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME" required:"true"`
		Topic       string `long:"pubsub-topic" description:"PubSub Topic to send message to" value-name:"NAME" required:"true"`
		PrivateKey  string `long:"pubsub-private-key" description:"path to file containing PEM-encoded, unencrypted RSA private key" required:"true"`
	} `group:"PubSub Options"`

	Metrics struct {
		SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN"`
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`
}

func main() {
	var opts Opts

	logger := lager.NewLogger("revok-ingestor")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Debug("starting")

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	if opts.Metrics.SentryDSN != "" {
		logger.RegisterSink(revok.NewSentrySink(opts.Metrics.SentryDSN, opts.Metrics.Environment))
	}

	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	generator := queue.NewGenerator()

	pubSubClient, err := pubsub.NewClient(context.Background(), opts.PubSub.ProjectName)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	topic := pubSubClient.Topic(opts.PubSub.Topic)

	privateKey, err := crypto.ReadRSAPrivateKey(opts.PubSub.PrivateKey)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	signer := crypto.NewRSASigner(privateKey)
	enqueuer := queue.NewPubSubEnqueuer(logger, topic, signer)
	in := ingestor.NewIngestor(enqueuer, emitter, "revok", generator)

	router := http.NewServeMux()
	router.Handle("/webhook", ingestor.NewHandler(logger, in, opts.GitHub.WebhookSecretTokens))
	router.Handle("/healthcheck", revok.ObliviousHealthCheck())

	members := []grouper.Member{
		{"api", http_server.New(fmt.Sprintf(":%d", opts.Port), router)},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	serverLogger := logger.Session("server", lager.Data{
		"port": opts.Port,
	})
	serverLogger.Info("starting")
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		serverLogger.Error("failed", err)
	}
}

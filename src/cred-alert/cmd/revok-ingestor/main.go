package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/net/context"

	"cred-alert/config"
	"cred-alert/crypto"
	"cred-alert/ingestor"
	"cred-alert/metrics"
	"cred-alert/queue"
	"cred-alert/revok"
)

func main() {
	var cfg *config.IngestorConfig
	var flagOpts config.IngestorOpts

	logger := lager.NewLogger("revok-ingestor")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Debug("starting")

	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	bs, err := ioutil.ReadFile(string(flagOpts.ConfigFile))
	if err != nil {
		logger.Error("failed-opening-config-file", err)
		os.Exit(1)
	}

	cfg, err = config.LoadIngestorConfig(bs)

	errs := cfg.Validate()
	if errs != nil {
		for _, err := range errs {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	}

	if cfg.IsSentryConfigured() {
		logger.RegisterSink(revok.NewSentrySink(cfg.Metrics.SentryDSN, cfg.Metrics.Environment))
	}

	emitter := metrics.BuildEmitter(cfg.Metrics.DatadogAPIKey, cfg.Metrics.Environment)
	generator := queue.NewGenerator()

	pubSubClient, err := pubsub.NewClient(context.Background(), cfg.PubSub.ProjectName)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	topic := pubSubClient.Topic(cfg.PubSub.Topic)

	privateKey, err := crypto.ReadRSAPrivateKey(string(cfg.PubSub.PrivateKeyPath))
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	signer := crypto.NewRSASigner(privateKey)
	enqueuer := queue.NewPubSubEnqueuer(logger, topic, signer)
	in := ingestor.NewIngestor(enqueuer, emitter, "revok", generator)

	clk := clock.NewClock()

	router := http.NewServeMux()
	router.Handle("/webhook", ingestor.NewHandler(logger, in, clk, emitter, cfg.GitHub.WebhookSecretTokens))
	router.Handle("/healthcheck", revok.ObliviousHealthCheck())

	certificate, err := config.LoadCertificate(
		cfg.Identity.CertificatePath,
		cfg.Identity.PrivateKeyPath,
		cfg.Identity.PrivateKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	caCertPool, err := config.LoadCertificatePool(cfg.Identity.CACertificatePath)
	if err != nil {
		log.Fatalln(err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      caCertPool,
	}

	apiServer := http_server.NewTLSServer(fmt.Sprintf(":%d", cfg.Port), router, tlsConfig)

	members := []grouper.Member{
		{"api", apiServer},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	serverLogger := logger.Session("server", lager.Data{
		"port": cfg.Port,
	})
	serverLogger.Info("starting")
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		serverLogger.Error("failed", err)
	}
}

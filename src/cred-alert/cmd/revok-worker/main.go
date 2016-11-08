package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"cloud.google.com/go/pubsub"

	"golang.org/x/net/trace"
	"golang.org/x/oauth2"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/queue"
	"cred-alert/revok"
	"cred-alert/revok/stats"
	"cred-alert/sniff"
)

type Opts struct {
	LogLevel                    string        `long:"log-level" description:"log level to use"`
	WorkDir                     string        `long:"work-dir" description:"directory to work in" value-name:"PATH" required:"true"`
	RepositoryDiscoveryInterval time.Duration `long:"repository-discovery-interval" description:"how frequently to ask GitHub for all repos to check which ones we need to clone and dirscan" required:"true" value-name:"SCAN_INTERVAL" default:"1h"`
	ChangeDiscoveryInterval     time.Duration `long:"change-discovery-interval" description:"how frequently to fetch changes for repositories on disk and scan the changes" required:"true" value-name:"SCAN_INTERVAL" default:"1h"`
	MinFetchInterval            time.Duration `long:"min-fetch-interval" description:"the minimum frequency to fetch changes for repositories on disk and scan the changes" value-name:"MIN_FETCH_INTERVAL" default:"6h"`
	MaxFetchInterval            time.Duration `long:"max-fetch-interval" description:"the maximum frequency to fetch changes for repositories on disk and scan the changes" value-name:"MAX_FETCH_INTERVAL" default:"168h"`

	Whitelist []string `short:"i" long:"ignore-pattern" description:"List of regex patterns to ignore." env:"IGNORED_PATTERNS" env-delim:"," value-name:"REGEX"`

	RPCBindIP   string `long:"rpc-server-bind-ip" default:"0.0.0.0" description:"IP address on which to listen for RPC traffic."`
	RPCBindPort uint16 `long:"rpc-server-bind-port" default:"50051" description:"Port on which to listen for RPC traffic."`

	GitHub struct {
		AccessToken    string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
		PrivateKeyPath string `long:"github-private-key-path" description:"private key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
		PublicKeyPath  string `long:"github-public-key-path" description:"public key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
	} `group:"GitHub Options"`

	PubSub struct {
		ProjectName string `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME" required:"true"`

		FetchHint struct {
			Subscription string `long:"fetch-hint-pubsub-subscription" description:"PubSub Topic receive messages from" value-name:"NAME" required:"true"`
		} `group:"PubSub Fetch Hint Options"`
	} `group:"PubSub Options"`

	Metrics struct {
		SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN"`
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`

	Slack struct {
		WebhookURL string `long:"slack-webhook-url" description:"Slack webhook URL" env:"SLACK_WEBHOOK_URL" value-name:"WEBHOOK"`
	} `group:"Slack Options"`

	MySQL struct {
		Username string `long:"mysql-username" description:"MySQL username" value-name:"USERNAME" required:"true"`
		Password string `long:"mysql-password" description:"MySQL password" value-name:"PASSWORD"`
		Hostname string `long:"mysql-hostname" description:"MySQL hostname" value-name:"HOSTNAME" required:"true"`
		Port     uint16 `long:"mysql-port" description:"MySQL port" value-name:"PORT" required:"true"`
		DBName   string `long:"mysql-dbname" description:"MySQL database name" value-name:"DBNAME" required:"true"`
	}

	RPC struct {
		ClientCACertificate string `long:"rpc-server-client-ca" description:"Path to client CA certificate" required:"true"`
		Certificate         string `long:"rpc-server-cert" description:"Path to RPC server certificate" required:"true"`
		PrivateKey          string `long:"rpc-server-private-key" description:"Path to RPC server private key" required:"true"`
	}
}

func main() {
	var opts Opts

	logger := lager.NewLogger("revok-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger.Debug("starting")

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if opts.Metrics.SentryDSN != "" {
		logger.RegisterSink(revok.NewSentrySink(opts.Metrics.SentryDSN, opts.Metrics.Environment))
	}

	workdir := opts.WorkDir
	_, err = os.Lstat(workdir)
	if err != nil {
		log.Fatalf("workdir error: %s", err)
	}

	githubHTTPClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: opts.GitHub.AccessToken},
			),
			Base: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}

	dbURI := db.NewDSN(opts.MySQL.Username, opts.MySQL.Password, opts.MySQL.DBName, opts.MySQL.Hostname, int(opts.MySQL.Port))
	database, err := migrations.LockDBAndMigrate(logger, "mysql", dbURI)
	if err != nil {
		log.Fatalf("db error: %s", err)
	}

	database.LogMode(false)

	clock := clock.NewClock()

	cloneMsgCh := make(chan revok.CloneMsg)
	ghClient := revok.NewGitHubClient(github.NewClient(githubHTTPClient))

	scanRepository := db.NewScanRepository(database, clock)
	repositoryRepository := db.NewRepositoryRepository(database)
	fetchRepository := db.NewFetchRepository(database)
	fetchIntervalUpdater := revok.NewFetchIntervalUpdater(
		repositoryRepository,
		opts.MinFetchInterval,
		opts.MaxFetchInterval,
	)
	credentialRepository := db.NewCredentialRepository(database)
	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	gitClient := gitclient.New(opts.GitHub.PrivateKeyPath, opts.GitHub.PublicKeyPath)
	repoWhitelist := notifications.BuildWhitelist(opts.Whitelist...)
	notifier := notifications.NewSlackNotifier(opts.Slack.WebhookURL, clock, repoWhitelist)
	sniffer := sniff.NewDefaultSniffer()
	ancestryScanner := revok.NewScanner(
		gitClient,
		repositoryRepository,
		scanRepository,
		credentialRepository,
		sniffer,
		notifier,
		emitter,
	)

	repoDiscoverer := revok.NewRepoDiscoverer(
		logger,
		workdir,
		cloneMsgCh,
		ghClient,
		clock,
		opts.RepositoryDiscoveryInterval,
		repositoryRepository,
	)

	cloner := revok.NewCloner(
		logger,
		workdir,
		cloneMsgCh,
		gitClient,
		repositoryRepository,
		ancestryScanner,
		emitter,
	)

	changeDiscoverer := revok.NewChangeDiscoverer(
		logger,
		gitClient,
		clock,
		opts.ChangeDiscoveryInterval,
		ancestryScanner,
		repositoryRepository,
		fetchRepository,
		fetchIntervalUpdater,
		emitter,
	)

	dirscanUpdater := revok.NewRescanner(
		logger,
		scanRepository,
		credentialRepository,
		ancestryScanner,
		notifier,
		emitter,
	)

	statsReporter := stats.NewReporter(
		logger,
		clock,
		60*time.Second,
		db.NewStatsRepository(database),
		emitter,
	)

	pushEventProcessor := revok.NewPushEventProcessor(
		changeDiscoverer,
		repositoryRepository,
	)

	headCredentialCounter := revok.NewHeadCredentialCounter(
		logger,
		repositoryRepository,
		clock,
		24*time.Hour,
		gitClient,
		sniffer,
	)

	certificate, err := tls.LoadX509KeyPair(
		opts.RPC.Certificate,
		opts.RPC.PrivateKey,
	)

	clientCertPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(opts.RPC.ClientCACertificate)
	if err != nil {
		log.Fatalf("failed to read client ca certificate: %s", err.Error())
	}

	ok := clientCertPool.AppendCertsFromPEM(bs)
	if !ok {
		log.Fatalf("failed to append client certs from pem: %s", err.Error())
	}

	grpcServer := revok.NewGRPCServer(
		logger,
		fmt.Sprintf("%s:%d", opts.RPCBindIP, opts.RPCBindPort),
		revok.NewRevokServer(logger, repositoryRepository),
		&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    clientCertPool,
		},
	)

	pubSubClient, err := pubsub.NewClient(context.Background(), opts.PubSub.ProjectName)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	hintSubscription := pubSubClient.Subscription(opts.PubSub.FetchHint.Subscription)

	failedMessageRepo := db.NewFailedMessageRepository(database)
	ack := queue.NewAcker()

	retryHandler := queue.NewRetryHandler(failedMessageRepo, pushEventProcessor, ack)

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"repo-discoverer", repoDiscoverer},
		{"cloner", cloner},
		{"change-discoverer", changeDiscoverer},
		{"dirscan-updater", dirscanUpdater},
		{"stats-reporter", statsReporter},
		{"github-hint-handler", queue.NewPubSubSubscriber(logger, hintSubscription, retryHandler)},
		{"head-credential-counter", headCredentialCounter},
		{"grpc-server", grpcServer},
		{"debug", http_server.New("127.0.0.1:6060", debugHandler())},
	}))

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatalf("failed-to-start: %s", err)
	}
}

func debugHandler() http.Handler {
	debugRouter := http.NewServeMux()
	debugRouter.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	debugRouter.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	debugRouter.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	debugRouter.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	debugRouter.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	debugRouter.HandleFunc("/debug/requests", func(w http.ResponseWriter, req *http.Request) {
		any, sensitive := trace.AuthRequest(req)
		if !any {
			http.Error(w, "not allowed", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		trace.Render(w, req, sensitive)
	})

	debugRouter.HandleFunc("/debug/events", func(w http.ResponseWriter, req *http.Request) {
		any, sensitive := trace.AuthRequest(req)
		if !any {
			http.Error(w, "not allowed", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		trace.RenderEvents(w, req, sensitive)
	})

	return debugRouter
}

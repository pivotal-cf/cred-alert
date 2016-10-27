package main

import (
	"context"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"cloud.google.com/go/pubsub"

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

	Whitelist []string `short:"i" long:"ignore-pattern" description:"List of regex patterns to ignore." env:"IGNORED_PATTERNS" env-delim:"," value-name:"REGEX"`

	GitHub struct {
		AccessToken    string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
		PrivateKeyPath string `long:"github-private-key-path" description:"private key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
		PublicKeyPath  string `long:"github-public-key-path" description:"public key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
	} `group:"GitHub Options"`

	PubSub struct {
		ProjectName string `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME" required:"true"`

		FetchHint struct {
			Subscription string `long:"fetch-hint-pubsub-subscription" description:"PubSub Topic recieve messages from" value-name:"NAME" required:"true"`
		} `group:"PubSub Fetch Hint Options"`
	} `group:"PubSub Options"`

	Metrics struct {
		SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN"`
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`

	Slack struct {
		WebhookUrl string `long:"slack-webhook-url" description:"Slack webhook URL" env:"SLACK_WEBHOOK_URL" value-name:"WEBHOOK"`
	} `group:"Slack Options"`

	MySQL struct {
		Username string `long:"mysql-username" description:"MySQL username" value-name:"USERNAME" required:"true"`
		Password string `long:"mysql-password" description:"MySQL password" value-name:"PASSWORD"`
		Hostname string `long:"mysql-hostname" description:"MySQL hostname" value-name:"HOSTNAME" required:"true"`
		Port     uint16 `long:"mysql-port" description:"MySQL port" value-name:"PORT" required:"true"`
		DBName   string `long:"mysql-dbname" description:"MySQL database name" value-name:"DBNAME" required:"true"`
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
	credentialRepository := db.NewCredentialRepository(database)
	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	gitClient := gitclient.New(opts.GitHub.PrivateKeyPath, opts.GitHub.PublicKeyPath)
	repoWhitelist := notifications.BuildWhitelist(opts.Whitelist...)
	notifier := notifications.NewSlackNotifier(opts.Slack.WebhookUrl, clock, repoWhitelist)
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

	pushEventProcessor := queue.NewPushEventProcessor(
		logger,
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

	pubSubClient, err := pubsub.NewClient(context.Background(), opts.PubSub.ProjectName)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}
	hintSubscription := pubSubClient.Subscription(opts.PubSub.FetchHint.Subscription)

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"repo-discoverer", repoDiscoverer},
		{"cloner", cloner},
		{"change-discoverer", changeDiscoverer},
		{"dirscan-updater", dirscanUpdater},
		{"stats-reporter", statsReporter},
		{"github-hint-handler", queue.NewPubSubSubscriber(logger, hintSubscription, pushEventProcessor)},
		{"head-credential-counter", headCredentialCounter},
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

	return debugRouter
}

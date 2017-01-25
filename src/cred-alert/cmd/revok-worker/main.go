package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/net/trace"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"

	"cred-alert/config"
	"cred-alert/crypto"
	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/queue"
	"cred-alert/revok"
	"cred-alert/revok/stats"
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/sniff"
	"red/redrunner"
)

func main() {
	var cfg *config.WorkerConfig
	var flagOpts config.WorkerOpts

	logger := lager.NewLogger("revok-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Debug("starting")

	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	if flagOpts.ConfigFile != "" {
		bs, err := ioutil.ReadFile(string(flagOpts.ConfigFile))
		if err != nil {
			logger.Error("failed-opening-config-file", err)
			os.Exit(1)
		}

		cfg, err = config.LoadWorkerConfig(bs)
		cfg.Merge(flagOpts.WorkerConfig)
	} else {
		cfg = flagOpts.WorkerConfig
	}

	errs := cfg.Validate()
	if errs != nil {
		for _, err := range errs {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	}

	if cfg.Metrics.SentryDSN != "" {
		logger.RegisterSink(revok.NewSentrySink(cfg.Metrics.SentryDSN, cfg.Metrics.Environment))
	}

	workdir := cfg.WorkDir
	_, err = os.Lstat(workdir)
	if err != nil {
		log.Fatalf("workdir error: %s", err)
	}

	dbURI := db.NewDSN(cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.DBName, cfg.MySQL.Hostname, int(cfg.MySQL.Port))
	database, err := migrations.LockDBAndMigrate(logger, "mysql", dbURI)
	if err != nil {
		log.Fatalf("db error: %s", err)
	}

	database.LogMode(false)

	clock := clock.NewClock()

	cloneMsgCh := make(chan revok.CloneMsg)

	scanRepository := db.NewScanRepository(database, clock)
	repositoryRepository := db.NewRepositoryRepository(database)
	fetchRepository := db.NewFetchRepository(database)
	credentialRepository := db.NewCredentialRepository(database)
	emitter := metrics.BuildEmitter(cfg.Metrics.DatadogAPIKey, cfg.Metrics.Environment)
	gitClient := gitclient.New(string(cfg.GitHub.PrivateKeyPath), string(cfg.GitHub.PublicKeyPath))
	repoWhitelist := notifications.BuildWhitelist(cfg.Whitelist...)

	var notifier notifications.Notifier
	if cfg.Slack.WebhookURL != "" {
		notifier = notifications.NewSlackNotifier(cfg.Slack.WebhookURL, clock, repoWhitelist)
	} else {
		notifier = notifications.NewNullNotifier()
	}

	sniffer := sniff.NewDefaultSniffer()
	ancestryScanner := revok.NewScanner(
		gitClient,
		repositoryRepository,
		scanRepository,
		credentialRepository,
		sniffer,
		notifier,
	)

	changeFetcher := revok.NewChangeFetcher(
		logger,
		gitClient,
		ancestryScanner,
		repositoryRepository,
		fetchRepository,
		emitter,
	)

	changeScheduleRunner := revok.NewScheduleRunner(logger)

	changeScheduler := revok.NewChangeScheduler(
		logger,
		repositoryRepository,
		changeScheduleRunner,
		changeFetcher,
	)

	cloner := revok.NewCloner(
		logger,
		workdir,
		cloneMsgCh,
		gitClient,
		repositoryRepository,
		ancestryScanner,
		emitter,
		changeScheduler,
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

	headCredentialCounter := revok.NewHeadCredentialCounter(
		logger,
		repositoryRepository,
		clock,
		cfg.CredentialCounterInterval,
		gitClient,
		sniffer,
	)

	members := []grouper.Member{
		{"cloner", cloner},
		{"dirscan-updater", dirscanUpdater},
		{"stats-reporter", statsReporter},
		{"head-credential-counter", headCredentialCounter},
		{"change-schedule-runner", changeScheduleRunner},
		{"debug", http_server.New("127.0.0.1:6060", debugHandler())},
	}

	if cfg.IsRPCConfigured() {
		certificate, err := config.LoadCertificate(
			string(cfg.RPC.CertificatePath),
			string(cfg.RPC.PrivateKeyPath),
			cfg.RPC.PrivateKeyPassphrase,
		)
		if err != nil {
			log.Fatalln(err)
		}

		clientCertPool, err := config.LoadCertificatePool(string(cfg.RPC.ClientCACertificatePath))
		if err != nil {
			log.Fatalln(err)
		}

		looper := gitclient.NewLooper()
		searcher := search.NewSearcher(repositoryRepository, looper)

		grpcServer := redrunner.NewGRPCServer(
			logger,
			fmt.Sprintf("%s:%d", cfg.RPC.BindIP, cfg.RPC.BindPort),
			&tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{certificate},
				ClientCAs:    clientCertPool,
			},
			func(server *grpc.Server) {
				revokpb.RegisterRevokServer(server, revok.NewServer(logger, repositoryRepository, searcher))
			},
		)

		members = append(members, grouper.Member{
			Name:   "grpc-server",
			Runner: grpcServer,
		})
	}

	if cfg.IsPubSubConfigured() {
		pubSubClient, err := pubsub.NewClient(context.Background(), cfg.PubSub.ProjectName)
		if err != nil {
			logger.Fatal("failed", err)
			os.Exit(1)
		}

		subscription := pubSubClient.Subscription(cfg.PubSub.FetchHint.Subscription)

		publicKey, err := crypto.ReadRSAPublicKey(string(cfg.PubSub.PublicKeyPath))
		if err != nil {
			logger.Fatal("failed", err)
			os.Exit(1)
		}
		pushEventProcessor := queue.NewPushEventProcessor(
			changeFetcher,
			crypto.NewRSAVerifier(publicKey),
			emitter,
		)

		members = append(members, grouper.Member{
			Name:   "github-hint-handler",
			Runner: queue.NewPubSubSubscriber(logger, subscription, pushEventProcessor),
		})
	}

	if cfg.GitHub.AccessToken != "" {
		githubHTTPClient := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &oauth2.Transport{
				Source: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: cfg.GitHub.AccessToken},
				),
				Base: &http.Transport{
					DisableKeepAlives: true,
				},
			},
		}

		ghClient := revok.NewGitHubClient(github.NewClient(githubHTTPClient))

		repoDiscoverer := revok.NewRepoDiscoverer(
			logger,
			workdir,
			cloneMsgCh,
			ghClient,
			clock,
			cfg.RepositoryDiscoveryInterval,
			repositoryRepository,
		)

		members = append(members, grouper.Member{
			Name:   "repo-discoverer",
			Runner: repoDiscoverer,
		})
	}

	startupTasks := []grouper.Member{
		{
			Name:   "schedule-fetches",
			Runner: changeScheduler,
		},
	}

	system := []grouper.Member{
		{
			Name:   "servers",
			Runner: grouper.NewParallel(os.Interrupt, members),
		},
		{
			Name:   "startup-tasks",
			Runner: grouper.NewParallel(os.Interrupt, startupTasks),
		},
	}

	runner := sigmon.New(grouper.NewOrdered(os.Interrupt, system))

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

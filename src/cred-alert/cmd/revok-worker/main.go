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
	"cred-alert/sniff"
)

func main() {
	var opts config.WorkerOpts

	logger := lager.NewLogger("revok-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger.Debug("starting")

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	errs := opts.Validate()
	if errs != nil {
		for _, err := range errs {
			fmt.Println(err.Error())
		}
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

	dbURI := db.NewDSN(opts.MySQL.Username, opts.MySQL.Password, opts.MySQL.DBName, opts.MySQL.Hostname, int(opts.MySQL.Port))
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
	fetchIntervalUpdater := revok.NewFetchIntervalUpdater(
		repositoryRepository,
		opts.MinFetchInterval,
		opts.MaxFetchInterval,
	)
	credentialRepository := db.NewCredentialRepository(database)
	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	gitClient := gitclient.New(opts.GitHub.PrivateKeyPath, opts.GitHub.PublicKeyPath)
	repoWhitelist := notifications.BuildWhitelist(opts.Whitelist...)

	var notifier notifications.Notifier
	if opts.Slack.WebhookURL != "" {
		notifier = notifications.NewSlackNotifier(opts.Slack.WebhookURL, clock, repoWhitelist)
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
		emitter,
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

	headCredentialCounter := revok.NewHeadCredentialCounter(
		logger,
		repositoryRepository,
		clock,
		opts.CredentialCounterInterval,
		gitClient,
		sniffer,
	)

	members := []grouper.Member{
		{"cloner", cloner},
		{"change-discoverer", changeDiscoverer},
		{"dirscan-updater", dirscanUpdater},
		{"stats-reporter", statsReporter},
		{"head-credential-counter", headCredentialCounter},
		{"debug", http_server.New("127.0.0.1:6060", debugHandler())},
	}

	if opts.IsRPCConfigured() {
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
			revok.NewServer(logger, repositoryRepository),
			&tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{certificate},
				ClientCAs:    clientCertPool,
			},
		)

		members = append(members, grouper.Member{
			Name:   "grpc-server",
			Runner: grpcServer,
		})
	}

	if opts.IsPubSubConfigured() {
		pubSubClient, err := pubsub.NewClient(context.Background(), opts.PubSub.ProjectName)
		if err != nil {
			logger.Fatal("failed", err)
			os.Exit(1)
		}

		subscription := pubSubClient.Subscription(opts.PubSub.FetchHint.Subscription)

		publicKey, err := crypto.ReadRSAPublicKey(opts.PubSub.PublicKey)
		if err != nil {
			logger.Fatal("failed", err)
			os.Exit(1)
		}
		pushEventProcessor := queue.NewPushEventProcessor(
			changeDiscoverer,
			repositoryRepository,
			crypto.NewRSAVerifier(publicKey),
			emitter,
		)

		members = append(members, grouper.Member{
			Name:   "github-hint-handler",
			Runner: queue.NewPubSubSubscriber(logger, subscription, pushEventProcessor),
		})
	}

	if opts.GitHub.AccessToken != "" {
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

		ghClient := revok.NewGitHubClient(github.NewClient(githubHTTPClient))

		repoDiscoverer := revok.NewRepoDiscoverer(
			logger,
			workdir,
			cloneMsgCh,
			ghClient,
			clock,
			opts.RepositoryDiscoveryInterval,
			repositoryRepository,
		)

		members = append(members, grouper.Member{
			Name:   "repo-discoverer",
			Runner: repoDiscoverer,
		})
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

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

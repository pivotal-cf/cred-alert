package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/trace"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/google/go-github/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/pivotal-cf/paraphernalia/operate/admin"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
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
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/sniff"
	"rolodex/rolodexpb"
)

var info = admin.ServiceInfo{
	Name:        "revok",
	Description: "A service which fetches new Git commits and scans them for credentials.",
	Team:        "PCF Security Enablement",
}

func main() {
	var cfg *config.WorkerConfig
	var flagOpts config.WorkerOpts

	logger := lager.NewLogger("revok-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger.Info("starting")

	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	bs, err := ioutil.ReadFile(string(flagOpts.ConfigFile))
	if err != nil {
		logger.Error("failed-to-open-config-file", err)
		os.Exit(1)
	}

	cfg, err = config.LoadWorkerConfig(bs)
	if err != nil {
		logger.Error("failed-to-load-config-file", err)
		os.Exit(1)
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

	dbCertificate, dbCaCertPool := loadCerts(
		cfg.MySQL.CertificatePath,
		cfg.MySQL.PrivateKeyPath,
		cfg.MySQL.PrivateKeyPassphrase,
		cfg.MySQL.CACertificatePath,
	)

	dbURI := db.NewDSN(
		cfg.MySQL.Username,
		cfg.MySQL.Password,
		cfg.MySQL.DBName,
		cfg.MySQL.Hostname,
		int(cfg.MySQL.Port),
		cfg.MySQL.ServerName,
		dbCertificate,
		dbCaCertPool,
	)

	database, err := migrations.LockDBAndMigrate(logger, "mysql", dbURI)
	if err != nil {
		log.Fatalf("db error: %s", err)
	}

	database.LogMode(false)

	clk := clock.NewClock()

	cloneMsgCh := make(chan revok.CloneMsg)

	scanRepository := db.NewScanRepository(database, clk)
	repositoryRepository := db.NewRepositoryRepository(database)
	fetchRepository := db.NewFetchRepository(database)
	credentialRepository := db.NewCredentialRepository(database)
	branchRepository := db.NewBranchRepository(database)

	emitter := metrics.BuildEmitter(cfg.Metrics.DatadogAPIKey, cfg.Metrics.Environment)
	gitClient := gitclient.New(cfg.GitHub.PrivateKeyPath, cfg.GitHub.PublicKeyPath)
	repoWhitelist := notifications.BuildWhitelist(cfg.Whitelist...)
	formatter := notifications.NewSlackNotificationFormatter()

	traceClient, err := trace.NewClient(context.Background(), cfg.Trace.ProjectName)
	if err != nil {
		logger.Error("failed-to-create-trace-client", err)
	}

	slackHTTPClient := &http.Client{
		Timeout: 5 * time.Second,
	}
	notifier := notifications.NewSlackNotifier(clk, slackHTTPClient, formatter)

	certificate, caCertPool := loadCerts(
		cfg.Identity.CertificatePath,
		cfg.Identity.PrivateKeyPath,
		cfg.Identity.PrivateKeyPassphrase,
		cfg.Identity.CACertificatePath,
	)

	rolodexServerAddr := fmt.Sprintf("%s:%d", cfg.Rolodex.ServerAddress, cfg.Rolodex.ServerPort)

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithPivotalDefaults(),
		tlsconfig.WithIdentity(certificate),
	)

	transportCreds := credentials.NewTLS(tlsConfig.Client(tlsconfig.WithAuthority(caCertPool)))

	conn, err := grpc.Dial(
		rolodexServerAddr,
		grpc.WithDialer(keepAliveDial),
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithUnaryInterceptor(traceClient.GRPCClientInterceptor()),
	)

	rolodexClient := rolodexpb.NewRolodexClient(conn)

	teamURLs := notifications.NewTeamURLs(
		cfg.Slack.DefaultURL,
		cfg.Slack.DefaultChannel,
		cfg.Slack.TeamURLs,
	)

	addressBook := notifications.NewRolodex(
		rolodexClient,
		teamURLs,
	)

	router := notifications.NewRouter(
		notifier,
		addressBook,
		repoWhitelist,
	)

	sniffer := sniff.NewDefaultSniffer()
	scanner := revok.NewScanner(
		gitClient,
		repositoryRepository,
		scanRepository,
		credentialRepository,
		sniffer,
	)

	notificationComposer := revok.NewNotificationComposer(
		repositoryRepository,
		router,
		scanner,
	)

	changeFetcher := revok.NewChangeFetcher(
		logger,
		gitClient,
		notificationComposer,
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
		notificationComposer,
		emitter,
		changeScheduler,
	)

	dirscanUpdater := revok.NewRescanner(
		logger,
		scanRepository,
		credentialRepository,
		scanner,
		router,
		emitter,
	)

	statsReporter := stats.NewReporter(
		logger,
		clk,
		60*time.Second,
		db.NewStatsRepository(database),
		emitter,
	)

	headCredentialCounter := revok.NewHeadCredentialCounter(
		logger,
		branchRepository,
		repositoryRepository,
		clk,
		cfg.CredentialCounterInterval,
		gitClient,
		sniffer,
	)

	debug := admin.Runner(
		"6060",
		admin.WithInfo(info),
		admin.WithUptime(),
	)

	members := []grouper.Member{
		{Name: "cloner", Runner: cloner},
		{Name: "dirscan-updater", Runner: dirscanUpdater},
		{Name: "stats-reporter", Runner: statsReporter},
		{Name: "head-credential-counter", Runner: headCredentialCounter},
		{Name: "change-schedule-runner", Runner: changeScheduleRunner},
		{Name: "debug", Runner: debug},
	}

	looper := gitclient.NewLooper()
	searcher := search.NewSearcher(repositoryRepository, looper)
	handler := revok.NewServer(logger, searcher, repositoryRepository, branchRepository)

	serverTls := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))

	grpcServer := grpcrunner.New(
		logger,
		fmt.Sprintf("%s:%d", cfg.API.BindIP, cfg.API.BindPort),
		func(server *grpc.Server) {
			revokpb.RegisterRevokServer(server, handler)
		},
		grpc.Creds(credentials.NewTLS(serverTls)),
	)

	members = append(members, grouper.Member{
		Name:   "grpc-server",
		Runner: grpcServer,
	})

	pubSubClient, err := pubsub.NewClient(context.Background(), cfg.PubSub.ProjectName)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	subscription := pubSubClient.Subscription(cfg.PubSub.FetchHint.Subscription)

	publicKey, err := crypto.ReadRSAPublicKey(cfg.PubSub.PublicKeyPath)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	pushEventProcessor := queue.NewPushEventProcessor(
		changeFetcher,
		emitter,
		clk,
		traceClient,
	)

	signatureChecker := queue.NewSignatureCheck(crypto.NewRSAVerifier(publicKey), emitter, pushEventProcessor)

	members = append(members, grouper.Member{
		Name:   "github-hint-handler",
		Runner: queue.NewPubSubSubscriber(logger, subscription, signatureChecker, emitter),
	})

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
			clk,
			cfg.RepositoryDiscovery.Interval,
			cfg.RepositoryDiscovery.Organizations,
			cfg.RepositoryDiscovery.Users,
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

func loadCerts(certificatePath, privateKeyPath, privateKeyPassphrase, caCertificatePath string) (tls.Certificate, *x509.CertPool) {
	certificate, err := config.LoadCertificate(
		certificatePath,
		privateKeyPath,
		privateKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	caCertPool, err := config.LoadCertificatePool(caCertificatePath)
	if err != nil {
		log.Fatalln(err)
	}

	return certificate, caCertPool
}

func keepAliveDial(addr string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{
		Timeout:   timeout,
		KeepAlive: 60 * time.Second,
	}
	return d.Dial("tcp", addr)
}

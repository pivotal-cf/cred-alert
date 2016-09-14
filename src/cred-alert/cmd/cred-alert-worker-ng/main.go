package main

import (
	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/revok"
	"cred-alert/sniff"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type Opts struct {
	LogLevel                    string        `long:"log-level" description:"log level to use"`
	WorkDir                     string        `long:"work-dir" description:"directory to work in" value-name:"PATH" required:"true"`
	RepositoryDiscoveryInterval time.Duration `long:"repository-discovery-interval" description:"how frequently to ask GitHub for all repos to check which ones we need to clone and dirscan" required:"true" value-name:"SCAN_INTERVAL" default:"1h"`
	ChangeDiscoveryInterval     time.Duration `long:"change-discovery-interval" description:"how frequently to fetch changes for repositories on disk and scan the changes" required:"true" value-name:"SCAN_INTERVAL" default:"1h"`

	GitHub struct {
		AccessToken    string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
		PrivateKeyPath string `long:"github-private-key-path" description:"private key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
		PublicKeyPath  string `long:"github-public-key-path" description:"public key to use for GitHub auth" required:"true" value-name:"SSH_KEY"`
	} `group:"GitHub Options"`

	Metrics struct {
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`

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

	logger := lager.NewLogger("cred-alert-worker-ng")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger.Debug("starting")

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
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
	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	gitClient := gitclient.New(opts.GitHub.PrivateKeyPath, opts.GitHub.PublicKeyPath)
	sniffer := sniff.NewDefaultSniffer()

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"repo-discoverer", revok.NewRepoDiscoverer(
			logger,
			workdir,
			cloneMsgCh,
			ghClient,
			clock,
			opts.RepositoryDiscoveryInterval,
			repositoryRepository,
		)},
		{"cloner", revok.NewCloner(
			logger,
			workdir,
			cloneMsgCh,
			gitClient,
			sniffer,
			repositoryRepository,
			scanRepository,
			emitter,
		)},
		{"change-discoverer", revok.NewChangeDiscoverer(
			logger,
			workdir,
			gitClient,
			clock,
			opts.ChangeDiscoveryInterval,
			sniffer,
			repositoryRepository,
			fetchRepository,
			scanRepository,
			emitter,
		)},
	}))
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatal("failed-to-start: %s", err)
	}
}

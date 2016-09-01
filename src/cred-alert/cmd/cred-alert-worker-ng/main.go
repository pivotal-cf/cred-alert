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
	"github.com/tedsuo/ifrit/sigmon"
)

type Opts struct {
	LogLevel     string        `long:"log-level" description:"log level to use"`
	WorkDir      string        `long:"work-dir" description:"directory to work in" value-name:"PATH" required:"true"`
	ScanInterval time.Duration `long:"scan-interval" description:"how frequently to ask GitHub for all repos to check which ones we need to clone/fetch and then scan" required:"true" value-name:"SCAN_INTERVAL" default:"1h"`

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

	runner := sigmon.New(workerRunner(logger, opts), os.Interrupt)
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatal("failed-to-start: %s", err)
	}
}

func workerRunner(logger lager.Logger, opts Opts) ifrit.Runner {
	workdir := opts.WorkDir
	_, err := os.Lstat(workdir)
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

	clock := clock.NewClock()

	return revok.New(
		logger,
		clock,
		workdir,
		github.NewClient(githubHTTPClient),
		gitclient.New(opts.GitHub.PrivateKeyPath, opts.GitHub.PublicKeyPath),
		sniff.NewDefaultSniffer(),
		opts.ScanInterval,
		db.NewScanRepository(database, clock),
		db.NewRepositoryRepository(database),
		db.NewFetchRepository(database),
		metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment),
	)
}

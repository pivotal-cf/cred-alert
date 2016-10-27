package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/oauth2"

	"cred-alert/metrics"
	"cred-alert/monitor"
)

type Opts struct {
	MonitoringInterval time.Duration `long:"monitoring-interval" description:"how frequently to poll GitHub remaining request limits" value-name:"MONITORING_INTERVAL" default:"1m" env:"MONITORING_INTERVAL"`

	GitHub struct {
		AccessToken string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
	} `group:"GitHub Options"`

	Metrics struct {
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`
}

func main() {
	var opts Opts

	logger := lager.NewLogger("stats-monitor")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Info("starting")

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		logger.Fatal("failed", err)
		os.Exit(1)
	}

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.GitHub.AccessToken},
	)

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &oauth2.Transport{
			Source: tokenSource,
			Base: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}

	clock := clock.NewClock()

	emitter := metrics.BuildEmitter(opts.Metrics.DatadogAPIKey, opts.Metrics.Environment)
	ghClient := github.NewClient(httpClient)
	mon := monitor.NewMonitor(logger, ghClient, emitter, clock, opts.MonitoringInterval)

	githubService := monitor.NewGithubService(logger)
	githubStatusMonitor := monitor.NewGithubMonitor(logger, githubService, clock, opts.MonitoringInterval, emitter)

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"github API monitor", mon},
		{"github status monitor", githubStatusMonitor},
	}))
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatalf("failed-to-start: %s", err)
	}
}

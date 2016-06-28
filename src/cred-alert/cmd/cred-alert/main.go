package main

import (
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/git"
	"cred-alert/github"
	"cred-alert/logging"
	"cred-alert/webhook"
)

type Opts struct {
	Port      uint16   `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`
	Whitelist []string `short:"i" long:"ignore-repos" description:"comma separated list of repo names to ignore. The names may be regex patterns." env:"IGNORED_REPOS" value-name:"REPOS_TO_IGNORE"`

	GitHub struct {
		WebhookToken string `short:"w" long:"webhook-token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_KEY" value-name:"TOKEN" required:"true"`
		AccessToken  string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" required:"true"`
	} `group:"GitHub Options"`

	Datadog struct {
		APIKey      string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATA_DOG_API_KEY" value-name:"KEY"`
		Environment string `long:"datadog-environment" description:"environment tag for datadog" env:"DATA_DOG_ENVIRONMENT_TAG" value-name:"NAME" default:"development"`
	} `group:"Datadog Options"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("cred-alert")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.GitHub.AccessToken},
	)
	httpClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	ghClient := github.NewClient(github.DEFAULT_GITHUB_URL, httpClient)

	emitter := logging.BuildEmitter(opts.Datadog.APIKey, opts.Datadog.Environment)
	eventHandler := webhook.NewEventHandler(ghClient, git.Scan, emitter, opts.Whitelist)

	router := http.NewServeMux()
	router.Handle("/webhook", webhook.Handler(logger, eventHandler, opts.GitHub.WebhookToken))

	members := []grouper.Member{
		{"api", http_server.New(
			fmt.Sprintf(":%d", opts.Port),
			router,
		)},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	logger.Info("starting-server", lager.Data{
		"port": opts.Port,
	})

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("running-server-failed", err)
	}
}

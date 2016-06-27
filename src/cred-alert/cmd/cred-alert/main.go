package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"

	"cred-alert/git"
	"cred-alert/github"
	"cred-alert/logging"
	"cred-alert/webhook"
)

type Opts struct {
	Port uint16 `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`

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

	logger.Info("starting-server", lager.Data{
		"port": opts.Port,
	})

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.GitHub.AccessToken},
	)
	httpClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	ghClient := github.NewClient(github.DEFAULT_GITHUB_URL, httpClient)

	emitter := logging.BuildEmitter(opts.Datadog.APIKey, opts.Datadog.Environment)
	scanner := webhook.NewPushEventScanner(ghClient, git.Scan, emitter)

	http.Handle("/webhook", webhook.Handler(logger, scanner, opts.GitHub.WebhookToken))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil))
}

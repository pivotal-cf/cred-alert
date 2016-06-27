package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/git"
	"cred-alert/logging"
	"cred-alert/webhook"
)

type Opts struct {
	Port uint16 `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`

	Token string `short:"t" long:"token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_KEY" value-name:"TOKEN" required:"true"`

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

	emitter := logging.BuildEmitter(opts.Datadog.APIKey, opts.Datadog.Environment)
	scanner := webhook.NewPushEventScanner(webhook.FetchDiff, git.Scan, emitter)

	logger.Info("starting-server", lager.Data{
		"port": opts.Port,
	})

	router := http.NewServeMux()
	router.Handle("/webhook", webhook.Handler(logger, scanner, opts.Token))

	members := []grouper.Member{
		{"api", http_server.New(
			fmt.Sprintf(":%d", opts.Port),
			router,
		)},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("running-app-failed", err)
	}
}

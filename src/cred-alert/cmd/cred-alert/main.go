package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"

	"cred-alert/webhook"
)

type Opts struct {
	Port uint16 `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT"`

	Token string `short:"t" long:"token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_KEY" value-name:"TOKEN" required:"true"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("cred-alert")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger.Info("starting-server", lager.Data{
		"port": opts.Port,
	})

	webhookHandler := webhook.HandleWebhook(logger, opts.Token)
	http.Handle("/webhook", webhookHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil))
}

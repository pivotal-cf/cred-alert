package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/pivotal-golang/lager"

	"cred-alert/webhook"
)

func main() {
	logger := lager.NewLogger("cred-alert")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	port := os.Getenv("PORT")
	secretKey := os.Getenv("GITHUB_WEBHOOK_SECRET_KEY")

	if secretKey == "" {
		logger.Error("environment-variable-missing", errors.New("GITHUB_WEBHOOK_SECRET_KEY not set"))
		os.Exit(1)
	}

	logger.Info("starting-server", lager.Data{
		"port": port,
	})

	webhookHandler := webhook.HandleWebhook(logger, secretKey)
	http.Handle("/webhook", webhookHandler)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

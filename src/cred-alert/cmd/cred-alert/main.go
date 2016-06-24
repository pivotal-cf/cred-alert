package main

import (
	"log"
	"net/http"
	"os"

	"github.com/pivotal-golang/lager"

	_ "cred-alert/logging"
	"cred-alert/webhook"
)

func main() {
	if os.Getenv("GITHUB_WEBHOOK_SECRET_KEY") == "" {
		log.Fatal("Error: environment variable GITHUB_WEBHOOK_SECRET_KEY not set")
	}
	webhook.SecretKey = []byte(os.Getenv("GITHUB_WEBHOOK_SECRET_KEY"))

	logger := lager.NewLogger("cred-alert")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	port := os.Getenv("PORT")

	logger.Info("starting-server", lager.Data{
		"port": port,
	})

	webhookHandler := webhook.HandleWebhook(logger)
	http.Handle("/webhook", webhookHandler)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

package main

import (
	"log"
	"net/http"
	"os"

	_ "cred-alert/logging"
	"cred-alert/webhook"
)

func main() {
	if os.Getenv("GITHUB_WEBHOOK_SECRET_KEY") == "" {
		log.Fatal("Error: environment variable GITHUB_WEBHOOK_SECRET_KEY not set")
	}
	if os.Getenv("GITHUB_ACCESS_TOKEN") == "" {
		log.Fatal("Error: environment variable GITHUB_ACCESS_TOKEN not set")
	}

	log.Print("Starting webserver...")

	secretKey := os.Getenv("GITHUB_WEBHOOK_SECRET_KEY")
	githubAccessToken := os.Getenv("GITHUB_ACCESS_TOKEN")

	webhookHandler := webhook.NewWebhookHandler(secretKey, githubAccessToken)
	http.HandleFunc("/webhook", webhookHandler.HandleWebhook)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

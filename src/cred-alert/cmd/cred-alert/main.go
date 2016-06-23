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
	webhook.SecretKey = []byte(os.Getenv("GITHUB_WEBHOOK_SECRET_KEY"))

	log.Print("Starting webserver...")

	http.HandleFunc("/webhook", webhook.HandleWebhook)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

package main

import (
	"cred-alert/webhook"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	if os.Getenv("GITHUB_WEBHOOK_SECRET_KEY") == "" {
		log.Fatal("Error: environment variable GITHUB_WEBHOOK_SECRET_KEY not set")
	}
	webhook.SecretKey = []byte(os.Getenv("GITHUB_WEBHOOK_SECRET_KEY"))

	fmt.Println("Starting webserver...")

	http.HandleFunc("/webhook", webhook.HandleWebhook)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

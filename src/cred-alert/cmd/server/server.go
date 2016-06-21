package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/github"
)

var SecretKey []byte

func main() {
	if os.Getenv("GITHUB_WEBHOOK_SECRET_KEY") == "" {
		log.Fatal("Error: environment variable GITHUB_WEBHOOK_SECRET_KEY not set")
	}
	SecretKey = []byte(os.Getenv("GITHUB_WEBHOOK_SECRET_KEY"))

	fmt.Println("Starting webserver...")

	http.HandleFunc("/webhook", WebhookFunc)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func WebhookFunc(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, SecretKey)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var event github.PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	myGithub "cred-alert/github"

	"github.com/google/go-github/github"
)

type WebhookHandler interface {
	HandleWebhook(w http.ResponseWriter, r *http.Request)
}

type webhookHandler struct {
	secretKey         []byte
	githubAccessToken string
}

func NewWebhookHandler(secretKey, githubAccessToken string) webhookHandler {
	return webhookHandler{
		secretKey:         []byte(secretKey),
		githubAccessToken: githubAccessToken,
	}
}

func (w webhookHandler) HandleWebhook(writer http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, w.secretKey)
	if err != nil {
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	var event github.PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	handlePushEvent(writer, event)
}

func handlePushEvent(w http.ResponseWriter, event github.PushEvent) {
	if event.Repo != nil && event.Repo.FullName != nil {
		fmt.Printf("Repo: %s\n", *event.Repo.FullName)
	}

	if event.Before == nil || *event.Before == "0000000000000000000000000000000000000000" || event.After == nil {
		fmt.Println("Push event is missing either a Before or After SHA")
		w.WriteHeader(http.StatusOK)
		return
	}

	fmt.Printf("Received a webhook. Before: %s, After: %s\n", *event.Before, *event.After)

	w.WriteHeader(http.StatusOK)

	githubClient := myGithub.DefaultClient()
	scanner := DefaultPushEventScanner(githubClient)
	go scanner.ScanPushEvent(event)
}

package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
)

var SecretKey []byte

func HandleWebhook(w http.ResponseWriter, r *http.Request) {
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

	fmt.Printf("Received a webhook. Before: %s, After: %s\n", *event.Before, *event.After)
	fmt.Printf("Owner: %s, Repo Name: %s\n", *event.Repo.Owner.Name, *event.Repo.Name)
	if event.Repo.FullName != nil {
		fmt.Printf("Repo Fullname: %s\n", *event.Repo.FullName)
	}
	w.WriteHeader(http.StatusOK)
}

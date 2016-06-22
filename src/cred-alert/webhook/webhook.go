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

	if event.Repo != nil {
		fmt.Printf("Owner: %s, Repo Name: %s\n", *event.Repo.Owner.Name, *event.Repo.Name)
	}

	if event.Before == nil || *event.Before == "0000000000000000000000000000000000000000" || event.After == nil {
		fmt.Println("Push event is missing either a Before or After")
		w.WriteHeader(http.StatusOK)
		return
	}

	fmt.Printf("Received a webhook. Before: %s, After: %s\n", *event.Before, *event.After)
	w.WriteHeader(http.StatusOK)
}

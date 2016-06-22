package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"cred-alert/git"
	myGithub "cred-alert/github"

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

	handlePushEvent(w, event)
}

func handlePushEvent(w http.ResponseWriter, event github.PushEvent) {
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

	go scanPushEvent(event)
}

func scanPushEvent(event github.PushEvent) {
	diff, err := fetchDiff(event)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request error: ", err)
	}

	lines := git.Scan(diff)
	for _, line := range lines {
		fmt.Printf("Found match in repo: %s, file: %s, After SHA: %s, line number: %d\n",
			*event.Repo.FullName,
			line.Path,
			*event.After,
			line.LineNumber)
	}
}

func fetchDiff(event github.PushEvent) (string, error) {
	httpClient := &http.Client{}
	githubClient := myGithub.NewClient("https://api.github.com/", httpClient)

	diff, err := githubClient.CompareRefs(*event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
	if err != nil {
		return "", err
	}

	return diff, nil
}

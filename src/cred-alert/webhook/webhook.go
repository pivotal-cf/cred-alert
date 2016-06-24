package webhook

import (
	"encoding/json"
	"net/http"

	myGithub "cred-alert/github"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

var SecretKey []byte

func HandleWebhook(logger lager.Logger) http.Handler {
	logger = logger.Session("webhook-handler")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, SecretKey)
		if err != nil {
			logger.Error("invalid-payload", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		var event github.PushEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			logger.Error("unmarshal-failed", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		handlePushEvent(logger, w, event)
	})
}

const initalCommitParentHash = "0000000000000000000000000000000000000000"

func handlePushEvent(logger lager.Logger, w http.ResponseWriter, event github.PushEvent) {
	logger = logger.Session("handling-push-event")

	if event.Repo != nil {
		logger = logger.WithData(lager.Data{
			"repo": *event.Repo.FullName,
		})
	}

	if event.Before == nil || *event.Before == initalCommitParentHash || event.After == nil {
		logger.Debug("event-missing-data")
		w.WriteHeader(http.StatusOK)
		return
	}

	logger.Info("handling-webhook-payload", lager.Data{
		"before": *event.Before,
		"after":  *event.After,
	})

	w.WriteHeader(http.StatusOK)

	scanner := DefaultPushEventScanner()
	go scanner.ScanPushEvent(logger, event)
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

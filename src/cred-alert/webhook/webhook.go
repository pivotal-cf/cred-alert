package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

const initalCommitParentHash = "0000000000000000000000000000000000000000"

type handler struct {
	logger       lager.Logger
	secretKey    []byte
	eventHandler EventHandler
}

func Handler(logger lager.Logger, eventHandler EventHandler, secretKey string) *handler {
	return &handler{
		logger:       logger.Session("webhook-handler"),
		secretKey:    []byte(secretKey),
		eventHandler: eventHandler,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.secretKey)
	if err != nil {
		h.logger.Error("invalid-payload", err)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var event github.PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		h.logger.Error("unmarshal-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.handlePushEvent(h.logger, w, event)
}

func (h *handler) handlePushEvent(logger lager.Logger, w http.ResponseWriter, event github.PushEvent) {
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

	go h.eventHandler.HandleEvent(logger, event)
}

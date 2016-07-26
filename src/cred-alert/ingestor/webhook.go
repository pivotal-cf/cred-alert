package ingestor

import (
	"encoding/json"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger    lager.Logger
	secretKey []byte
	ingestor  Ingestor
}

func Handler(logger lager.Logger, ingestor Ingestor, secretKey string) *handler {
	return &handler{
		logger:    logger.Session("webhook-handler"),
		secretKey: []byte(secretKey),
		ingestor:  ingestor,
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

	err = h.handlePushEvent(h.logger, w, event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *handler) handlePushEvent(logger lager.Logger, w http.ResponseWriter, event github.PushEvent) error {
	logger = logger.Session("handling-push-event")

	scan, valid := Extract(event)
	if !valid {
		if event.Repo != nil && event.Repo.FullName != nil {
			logger = logger.WithData(lager.Data{
				"repo": *event.Repo.FullName,
			})
		}

		logger.Debug("event-missing-data")
		return nil
	}

	logger.Info("handling-webhook-payload", lager.Data{
		"repo":   scan.FullRepoName(),
		"before": scan.From,
		"after":  scan.To,
	})

	return h.ingestor.IngestPushScan(logger, scan)
}

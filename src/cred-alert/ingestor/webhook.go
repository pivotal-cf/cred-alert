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

	scan, valid := Extract(event)
	if !valid {
		h.logger.Debug("event-missing-data", lager.Data{
			"repo": *event.Repo.FullName,
		})
		w.WriteHeader(http.StatusOK)
		return
	}

	h.logger.Info("handling-webhook-payload", lager.Data{
		"before":  scan.From,
		"after":   scan.To,
		"owner":   scan.Owner,
		"repo":    scan.Repository,
		"private": scan.Private,
	})

	err = h.ingestor.IngestPushScan(h.logger, scan, r.Header.Get("X-GitHub-Delivery"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

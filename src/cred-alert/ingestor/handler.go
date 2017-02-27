package ingestor

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
)

type handler struct {
	logger     lager.Logger
	secretKeys []string
	ingestor   Ingestor
}

func NewHandler(logger lager.Logger, ingestor Ingestor, secretKeys []string) http.Handler {
	return &handler{
		logger:     logger.Session("webhook-handler"),
		secretKeys: secretKeys,
		ingestor:   ingestor,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("starting")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("invalid-payload", err)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	r.Body.Close()

	var payload []byte
	for i, secretKey := range h.secretKeys {
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
		payload, err = github.ValidatePayload(r, []byte(secretKey))
		if err == nil {
			break
		}

		if i == len(h.secretKeys)-1 {
			h.logger.Error("invalid-payload", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var event github.PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		h.logger.Error("unmarshal-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if event.Before == nil || event.After == nil {
		h.logger.Info("invalid-event-dropped")
		w.WriteHeader(http.StatusOK)
		return
	}

	scan := PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		From:       *event.Before,
		To:         *event.After,
		Private:    *event.Repo.Private,
		PushTime:   event.Repo.PushedAt.Time,
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
		h.logger.Error("ingest-push-scan-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.logger.Debug("done")
	w.WriteHeader(http.StatusOK)
}

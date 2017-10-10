package ingestor

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"

	"cred-alert/metrics"
)

type handler struct {
	logger            lager.Logger
	secretKeys        []string
	ingestor          Ingestor
	clock             clock.Clock
	webhookDelayGauge metrics.Gauge
}

func NewHandler(logger lager.Logger, ingestor Ingestor, clock clock.Clock, emitter metrics.Emitter, secretKeys []string) http.Handler {
	return &handler{
		logger:            logger.Session("webhook-handler"),
		secretKeys:        secretKeys,
		ingestor:          ingestor,
		clock:             clock,
		webhookDelayGauge: emitter.Gauge("ingestor.webhook-delay"),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := h.clock.Now()

	h.logger.Debug("starting")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("invalid-payload", err)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	r.Body.Close()
	r.Header.Set("Content-Type", "application/json")

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

	delay := now.Sub(event.Repo.PushedAt.Time).Seconds()
	h.webhookDelayGauge.Update(h.logger, float32(delay))

	h.logger.Info("handling-webhook-payload", lager.Data{
		"before":    *event.Before,
		"after":     *event.After,
		"owner":     *event.Repo.Owner.Name,
		"repo":      *event.Repo.Name,
		"private":   *event.Repo.Private,
		"github-id": r.Header.Get("X-GitHub-Delivery"),
	})

	scan := PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		PushTime:   now,
	}

	err = h.ingestor.IngestPushScan(h.logger, scan)
	if err != nil {
		h.logger.Error("ingest-push-scan-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.logger.Debug("done")
	w.WriteHeader(http.StatusOK)
}

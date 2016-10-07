package queue

import (
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
)

type httpEnqueuer struct {
	logger lager.Logger
	url    string
}

func NewHTTPEnqueuer(logger lager.Logger, url string) Enqueuer {
	return &httpEnqueuer{logger: logger, url: url}
}

func (h *httpEnqueuer) Enqueue(task Task) error {
	reader := strings.NewReader(task.Payload())
	resp, err := http.Post(h.url, "application/json", reader)
	if err != nil {
		h.logger.Error("failed-to-enqueue", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		h.logger.Info("received-bad-response", lager.Data{
			"status_code": resp.StatusCode,
		})
	}

	return nil
}

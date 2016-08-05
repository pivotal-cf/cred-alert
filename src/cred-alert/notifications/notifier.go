package notifications

import (
	"bytes"
	"cred-alert/scanners"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	SendNotification(logger lager.Logger, repository string, sha string, line scanners.Line, private bool) error
}

type slackNotifier struct {
	webhookURL string
	client     *http.Client
	clock      clock.Clock
}

type slackNotification struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Fallback string `json:"fallback"`
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

func NewSlackNotifier(webhookURL string, clock clock.Clock) Notifier {
	if webhookURL == "" {
		return &nullSlackNotifier{}
	}

	return &slackNotifier{
		webhookURL: webhookURL,
		clock:      clock,
		client: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

const maxRetries = 3

func (n *slackNotifier) SendNotification(logger lager.Logger, repository string, sha string, line scanners.Line, private bool) error {
	logger = logger.Session("send-notification")
	logger.Debug("starting")

	notification := n.buildNotification(repository, sha, line, private)

	body, err := json.Marshal(notification)
	if err != nil {
		logger.Error("unmarshal-faiiled", err)
		return err
	}

	for numReq := 0; numReq < maxRetries; numReq++ {
		req, err := http.NewRequest("POST", n.webhookURL, bytes.NewBuffer(body))
		if err != nil {
			logger.Error("request-failed", err)
			return err
		}

		req.Header.Set("Content-type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			logger.Error("response-error", err)
			return err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			logger.Debug("done")
			return nil
		case http.StatusTooManyRequests:
			lastLoop := (numReq == maxRetries-1)
			if lastLoop {
				break
			}

			afterStr := resp.Header.Get("Retry-After")
			after, err := strconv.Atoi(afterStr)
			if err != nil {
				logger.Error("failed", err)
				return err
			}

			wait := after + 1 // +1 for luck

			n.clock.Sleep(time.Duration(wait) * time.Second)
			continue
		default:
			err = fmt.Errorf("bad response (!200): %d", resp.StatusCode)
			logger.Error("bad-response", err)
			return err
		}
	}

	err = errors.New("retried too many times")
	logger.Error("failed", err)

	return err
}

func (n *slackNotifier) buildNotification(repository string, sha string, line scanners.Line, private bool) slackNotification {
	link := fmt.Sprintf("https://github.com/%s/blob/%s/%s#L%d", repository, sha, line.Path, line.LineNumber)

	color := "danger"
	if private {
		color = "warning"
	}
	return slackNotification{
		Attachments: []slackAttachment{
			{
				Fallback: link,
				Color:    color,
				Title:    fmt.Sprintf("Credential detected in %s!", repository),
				Text:     fmt.Sprintf("<%s|%s:%d>", link, line.Path, line.LineNumber),
			},
		},
	}
}

type nullSlackNotifier struct{}

func (n *nullSlackNotifier) SendNotification(logger lager.Logger, repository string, sha string, line scanners.Line, private bool) error {
	logger.Session("send-notification").Debug("done")
	return nil
}

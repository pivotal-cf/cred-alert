package notifications

import (
	"bytes"
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
	SendNotification(lager.Logger, Notification) error
}

type Notification struct {
	Owner      string
	Repository string
	Private    bool

	SHA string

	Path       string
	LineNumber int
}

func (n Notification) FullName() string {
	return fmt.Sprintf("%s/%s", n.Owner, n.Repository)
}

type slackNotifier struct {
	webhookURL string
	client     *http.Client
	clock      clock.Clock
}

type slackMessage struct {
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

func (n *slackNotifier) SendNotification(logger lager.Logger, notification Notification) error {
	logger = logger.Session("send-notification")
	logger.Debug("starting")

	message := n.formatSlackMessage(notification)

	body, err := json.Marshal(message)
	if err != nil {
		logger.Error("unmarshal-faiiled", err)
		return err
	}

	return n.send(logger, body)
}

func (n *slackNotifier) send(logger lager.Logger, body []byte) error {
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

	err := errors.New("retried too many times")
	logger.Error("failed", err)

	return err
}

func (n *slackNotifier) formatSlackMessage(not Notification) slackMessage {
	link := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s#L%d", not.Owner, not.Repository, not.SHA, not.Path, not.LineNumber)

	color := "danger"
	if not.Private {
		color = "warning"
	}

	return slackMessage{
		Attachments: []slackAttachment{
			{
				Fallback: link,
				Color:    color,
				Title:    fmt.Sprintf("Credential detected in %s!", not.FullName()),
				Text:     fmt.Sprintf("<%s|%s:%d>", link, not.Path, not.LineNumber),
			},
		},
	}
}

type nullSlackNotifier struct{}

func (n *nullSlackNotifier) SendNotification(logger lager.Logger, notification Notification) error {
	logger.Session("send-notification").Debug("done")

	return nil
}

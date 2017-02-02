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

type SlackMessage struct {
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Fallback string `json:"fallback"`
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

type slackNotifier struct {
	webhookURL string

	client    *http.Client
	clock     clock.Clock
	whitelist Whitelist
	formatter SlackNotificationFormatter
}

func NewSlackNotifier(webhookURL string, clock clock.Clock, whitelist Whitelist, formatter SlackNotificationFormatter) Notifier {
	return &slackNotifier{
		webhookURL: webhookURL,
		clock:      clock,
		whitelist:  whitelist,
		formatter:  formatter,
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

	return n.SendBatchNotification(logger, []Notification{notification})
}

func (n *slackNotifier) SendBatchNotification(logger lager.Logger, batch []Notification) error {
	logger = logger.Session("send-batch-notification", lager.Data{"batch-size": len(batch)})
	logger.Debug("starting")

	if len(batch) == 0 {
		logger.Debug("done")
		return nil
	}

	filteredBatch := n.filterMessages(batch)
	messages := n.formatter.FormatNotifications(filteredBatch)

	for _, message := range messages {
		body, err := json.Marshal(message)
		if err != nil {
			logger.Error("unmarshal-faiiled", err)
			return err
		}

		err = n.send(logger, body)
		if err != nil {
			return err
		}
	}

	return nil
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
			logger.Info("told-to-wait", lager.Data{"after": afterStr})
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

func (n *slackNotifier) filterMessages(batch []Notification) []Notification {
	filtered := []Notification{}

	for _, notification := range batch {
		if !n.whitelist.ShouldSkipNotification(notification.Private, notification.Repository) {
			filtered = append(filtered, notification)
		}
	}

	return filtered
}

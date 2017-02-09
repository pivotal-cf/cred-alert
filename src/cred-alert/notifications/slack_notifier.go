package notifications

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Fallback string `json:"fallback"`
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

type slackNotifier struct {
	client    *http.Client
	clock     clock.Clock
	formatter SlackNotificationFormatter
}

func NewSlackNotifier(clock clock.Clock, formatter SlackNotificationFormatter) Notifier {
	return &slackNotifier{
		clock:     clock,
		formatter: formatter,
		client: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

const maxRetries = 3

func (n *slackNotifier) Send(logger lager.Logger, envelope Envelope) error {
	logger = logger.Session("send-notification", lager.Data{
		"channel": envelope.Address.Channel,
	})

	logger.Debug("starting")
	messages := n.formatter.FormatNotifications(envelope.Contents)

	for _, message := range messages {
		if envelope.Address.Channel != "" {
			message.Channel = fmt.Sprintf("#%s", envelope.Address.Channel)
		}

		body, err := json.Marshal(message)
		if err != nil {
			logger.Error("unmarshal-failed", err)
			return err
		}

		err = n.send(logger, envelope.Address.URL, body)
		if err != nil {
			return err
		}
	}

	logger.Debug("done")

	return nil
}

func (n *slackNotifier) send(logger lager.Logger, url string, body []byte) error {
	for numReq := 0; numReq < maxRetries; numReq++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
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
			return nil
		case http.StatusTooManyRequests:
			lastLoop := numReq == maxRetries-1
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
			message, _ := ioutil.ReadAll(resp.Body)
			logger.Error("bad-response", err, lager.Data{
				"body": string(message),
			})
			return err
		}
	}

	err := errors.New("retried too many times")
	logger.Error("failed", err)

	return err
}

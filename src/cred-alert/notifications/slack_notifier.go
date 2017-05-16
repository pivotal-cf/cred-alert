package notifications

import (
	"bytes"
	"context"
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
	client    HTTPClient
	clock     clock.Clock
	formatter SlackNotificationFormatter
}

//go:generate counterfeiter . HTTPClient

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

func NewSlackNotifier(clock clock.Clock, client HTTPClient, formatter SlackNotificationFormatter) Notifier {
	return &slackNotifier{
		clock:     clock,
		formatter: formatter,
		client:    client,
	}
}

const maxAttempts = 3

func (n *slackNotifier) Send(ctx context.Context, logger lager.Logger, envelope Envelope) error {
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

		err = n.send(ctx, logger, envelope.Address.URL, body)
		if err != nil {
			return err
		}
	}

	logger.Debug("done")

	return nil
}

func (n *slackNotifier) send(ctx context.Context, logger lager.Logger, url string, body []byte) error {
	var sendErr error

	for attempts := 0; attempts < maxAttempts; attempts++ {
		retry, err := n.makeSingleAttempt(ctx, logger, attempts, url, body)
		if err == nil {
			return nil
		}

		if !retry {
			return err
		}

		sendErr = err
	}

	err := fmt.Errorf("retried too many times: %s", sendErr)
	logger.Error("failed", err)

	return err
}

func (n *slackNotifier) makeSingleAttempt(ctx context.Context, logger lager.Logger, currentAttempt int, url string, body []byte) (bool, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		logger.Error("request-creation-failed", err)
		return false, err
	}
	req.Header.Set("Content-type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		if isTemporary(err) {
			return true, err
		}

		logger.Error("fatal-transport-error", err)
		return false, err
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return false, nil
	case http.StatusTooManyRequests:
		err := errors.New("slack applied back pressure")

		lastLoop := currentAttempt == maxAttempts-1
		if lastLoop {
			return true, err
		}

		if err := n.handleSlackBackPressure(logger, resp); err != nil {
			return false, err
		}

		return true, err
	default:
		err = fmt.Errorf("bad response (!200): %d", resp.StatusCode)
		message, _ := ioutil.ReadAll(resp.Body)
		logger.Error("response-error", err, lager.Data{
			"body": string(message),
		})
		return false, err
	}
}

func (n *slackNotifier) handleSlackBackPressure(logger lager.Logger, resp *http.Response) error {
	afterStr := resp.Header.Get("Retry-After")
	logger.Info("told-to-wait", lager.Data{"after": afterStr})
	after, err := strconv.Atoi(afterStr)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	wait := after + 1 // +1 for luck
	n.clock.Sleep(time.Duration(wait) * time.Second)

	return nil
}

type temporary interface {
	Temporary() bool
}

func isTemporary(err error) bool {
	t, ok := err.(temporary)
	return ok && t.Temporary()
}

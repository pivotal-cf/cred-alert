package notifications

import (
	"bytes"
	"cred-alert/sniff"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	SendNotification(logger lager.Logger, repository string, sha string, line sniff.Line) error
}

type slackNotifier struct {
	logger lager.Logger

	webhookURL string
	client     *http.Client
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

func NewSlackNotifier(webhookURL string) Notifier {
	if webhookURL == "" {
		return &nullSlackNotifier{}
	}

	return &slackNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 1 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

func (n *slackNotifier) SendNotification(logger lager.Logger, repository string, sha string, line sniff.Line) error {
	logger = logger.Session("send-notification")

	notification := n.buildNotification(repository, sha, line)

	body, err := json.Marshal(notification)
	if err != nil {
		logger.Error("unmarshal-faiiled", err)
		return err
	}

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

	if resp.StatusCode != http.StatusOK {
		logger.Info(fmt.Sprintf("bad responze (!200): %d", resp.StatusCode))
		return fmt.Errorf("bad responze (!200): %d", resp.StatusCode)
	}

	return nil
}

func (n *slackNotifier) buildNotification(repository string, sha string, line sniff.Line) slackNotification {
	link := fmt.Sprintf("https://github.com/%s/blob/%s/%s#L%d", repository, sha, line.Path, line.LineNumber)

	return slackNotification{
		Attachments: []slackAttachment{
			{
				Fallback: link,
				Color:    "danger",
				Title:    fmt.Sprintf("Credential detected in %s!", repository),
				Text:     fmt.Sprintf("<%s|%s:%d>", link, line.Path, line.LineNumber),
			},
		},
	}
}

type nullSlackNotifier struct {
	logger lager.Logger
}

func (n *nullSlackNotifier) SendNotification(logger lager.Logger, repository string, sha string, line sniff.Line) error {
	logger.Session("send-notification").Debug("sent")

	return nil
}

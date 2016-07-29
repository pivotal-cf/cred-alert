package notifications

import (
	"bytes"
	"cred-alert/scanners"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	SendNotification(logger lager.Logger, repository string, sha string, line scanners.Line, private bool) error
}

type slackNotifier struct {
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
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

func (n *slackNotifier) SendNotification(logger lager.Logger, repository string, sha string, line scanners.Line, private bool) error {
	logger = logger.Session("send-notification")
	logger.Info("starting")

	notification := n.buildNotification(repository, sha, line, private)

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
		err := fmt.Errorf("bad response (!200): %d", resp.StatusCode)
		logger.Error("bad-response", err)
		return err
	}

	logger.Debug("done")
	return nil
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

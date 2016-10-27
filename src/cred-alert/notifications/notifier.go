package notifications

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	SendNotification(lager.Logger, Notification) error
	SendBatchNotification(lager.Logger, []Notification) error
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

func (n Notification) ShortSHA() string {
	return n.SHA[:7]
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

type slackNotifier struct {
	webhookURL string
	client     *http.Client
	clock      clock.Clock
	whitelist  Whitelist
}

func NewSlackNotifier(webhookURL string, clock clock.Clock, whitelist Whitelist) Notifier {
	if webhookURL == "" {
		return &nullSlackNotifier{}
	}

	return &slackNotifier{
		webhookURL: webhookURL,
		clock:      clock,
		whitelist:  whitelist,
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

	messages := n.formatBatchSlackMessages(batch)

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

type slackLink struct {
	Text string
	Href string
}

func (l slackLink) String() string {
	return fmt.Sprintf("<%s|%s>", l.Href, l.Text)
}

type slackBatchRepo struct {
	Owner      string
	Repository string
	SHA        string
	Private    bool
}

func (r slackBatchRepo) FullName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Repository)
}

func (r slackBatchRepo) ShortSHA() string {
	return r.SHA[:7]
}

func (n *slackNotifier) formatBatchSlackMessages(batch []Notification) []slackMessage {
	messages := []slackMessage{}

	messageMap := make(map[slackBatchRepo]map[string][]Notification)
	repos := []slackBatchRepo{}

	for _, note := range batch {
		if n.whitelist.ShouldSkipNotification(note.Private, note.Repository) {
			continue
		}

		repo := slackBatchRepo{
			Owner:      note.Owner,
			Repository: note.Repository,
			SHA:        note.SHA,
			Private:    note.Private,
		}

		_, found := messageMap[repo]
		if !found {
			repos = append(repos, repo)
			messageMap[repo] = make(map[string][]Notification)
		}

		messageMap[repo][note.Path] = append(messageMap[repo][note.Path], note)
	}

	for _, repo := range repos {
		files := messageMap[repo]
		commitLink := fmt.Sprintf("https://github.com/%s/%s/commit/%s", repo.Owner, repo.Repository, repo.SHA)
		title := fmt.Sprintf("Possible credentials found in %s!", slackLink{
			Text: fmt.Sprintf("%s / %s", repo.FullName(), repo.ShortSHA()),
			Href: commitLink,
		})
		fallback := fmt.Sprintf("Possible credentials found in %s!", commitLink)

		color := "danger"
		if repo.Private {
			color = "warning"
		}

		// Make sure we get a consistent map iteration order.
		fileNames := []string{}
		for path := range files {
			fileNames = append(fileNames, path)
		}
		sort.Strings(fileNames)

		fileLines := []string{}

		for _, path := range fileNames {
			nots := files[path]
			fileLink := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", repo.Owner, repo.Repository, repo.SHA, path)

			lineLinks := []string{}

			for _, not := range nots {
				lineLink := fmt.Sprintf("%s#L%d", fileLink, not.LineNumber)

				lineLinks = append(lineLinks, slackLink{
					Text: strconv.Itoa(not.LineNumber),
					Href: lineLink,
				}.String())
			}

			plurality := "line"
			if len(lineLinks) > 1 {
				plurality = "lines"
			}

			text := fmt.Sprintf("• %s on %s %s", slackLink{
				Text: path,
				Href: fileLink,
			}, plurality, humanizeList(lineLinks))

			fileLines = append(fileLines, text)
		}

		messages = append(messages, slackMessage{
			Attachments: []slackAttachment{
				{
					Title:    title,
					Text:     strings.Join(fileLines, "\n"),
					Color:    color,
					Fallback: fallback,
				},
			},
		})
	}

	return messages
}

func humanizeList(list []string) string {
	joinedLines := &bytes.Buffer{}

	if len(list) <= 1 {
		joinedLines.WriteString(list[0])
	} else if len(list) == 2 {
		joinedLines.WriteString(list[0])
		joinedLines.WriteString(" and ")
		joinedLines.WriteString(list[1])
	} else {
		for _, line := range list[:len(list)-1] {
			joinedLines.WriteString(line)
			joinedLines.WriteString(", ")
		}

		joinedLines.WriteString("and ")
		joinedLines.WriteString(list[len(list)-1])
	}

	return joinedLines.String()
}

type nullSlackNotifier struct{}

func (n *nullSlackNotifier) SendNotification(logger lager.Logger, notification Notification) error {
	logger.Session("send-notification").Debug("done")

	return nil
}

func (n *nullSlackNotifier) SendBatchNotification(logger lager.Logger, batch []Notification) error {
	logger.Session("send-batch-notification").Debug("done")

	return nil
}

package webhook

import (
	"cred-alert/git"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"errors"
	"fmt"
	"regexp"

	gh "cred-alert/github"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . EventHandler

type EventHandler interface {
	HandleEvent(lager.Logger, github.PushEvent)
}

type eventHandler struct {
	githubClient gh.Client
	scan         func(lager.Logger, string) []git.Line
	whitelist    []*regexp.Regexp

	requestCounter    metrics.Counter
	credentialCounter metrics.Counter
	notifier          notifications.Notifier
}

func NewEventHandler(githubClient gh.Client, scan func(lager.Logger, string) []git.Line, emitter metrics.Emitter, notifier notifications.Notifier, whitelist []string) *eventHandler {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	credentialCounter := emitter.Counter("cred_alert.violations")

	patterns := make([]*regexp.Regexp, len(whitelist))
	for i, uncompiled := range whitelist {
		patterns[i] = regexp.MustCompile(uncompiled)
	}

	handler := &eventHandler{
		githubClient: githubClient,
		scan:         scan,
		whitelist:    patterns,

		requestCounter:    requestCounter,
		credentialCounter: credentialCounter,
		notifier:          notifier,
	}

	return handler
}

func (s *eventHandler) HandleEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("handle-event")

	if s.isWhitelisted(event) {
		logger.Info("ignored-repo", lager.Data{
			"repo": *event.Repo.Name,
		})
		return
	}

	s.requestCounter.Inc(logger)

	previousSHA := *event.Before
	violations := 0

	for _, commit := range event.Commits {
		if commit.ID == nil {
			continue
		}
		currentSHA := *commit.ID

		diff, err := s.githubClient.CompareRefs(logger, *event.Repo.Owner.Name, *event.Repo.Name, previousSHA, currentSHA)
		if err != nil {
			logger.Error("failed-fetch-diff", errors.New("Couldn't fetch diff "+previousSHA+" "+currentSHA))
			continue
		}

		lines := s.scan(logger, diff)
		for _, line := range lines {
			logger.Info("found-credential", lager.Data{
				"path":        line.Path,
				"line-number": line.LineNumber,
				"sha":         currentSHA,
			})
			s.notifier.SendNotification(fmt.Sprintf("Found credential in %s\n\tCommit SHA: %s\n\tFile: %s:%d\n", *event.Repo.FullName, currentSHA, line.Path, line.LineNumber))
			violations++
		}

		previousSHA = currentSHA
	}

	if violations > 0 {
		s.credentialCounter.IncN(logger, violations)
	}
}

func (s *eventHandler) isWhitelisted(event github.PushEvent) bool {
	if event.Repo.Name == nil {
		return false
	}

	for _, pattern := range s.whitelist {
		if pattern.MatchString(*event.Repo.Name) {
			return true
		}
	}

	return false
}

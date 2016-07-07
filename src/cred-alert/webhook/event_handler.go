package webhook

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners/git"
	"cred-alert/sniff"
	"errors"

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
	sniff        func(lager.Logger, sniff.Scanner, func(sniff.Line))
	whitelist    *Whitelist

	requestCounter      metrics.Counter
	credentialCounter   metrics.Counter
	ignoredEventCounter metrics.Counter
	notifier            notifications.Notifier
}

func NewEventHandler(githubClient gh.Client, sniff func(lager.Logger, sniff.Scanner, func(sniff.Line)), emitter metrics.Emitter, notifier notifications.Notifier, whitelist *Whitelist) *eventHandler {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	credentialCounter := emitter.Counter("cred_alert.violations")
	ignoredEventCounter := emitter.Counter("cred_alert.ignored_events")

	handler := &eventHandler{
		githubClient: githubClient,
		sniff:        sniff,
		whitelist:    whitelist,

		requestCounter:      requestCounter,
		credentialCounter:   credentialCounter,
		ignoredEventCounter: ignoredEventCounter,
		notifier:            notifier,
	}

	return handler
}

func (s *eventHandler) HandleEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("handle-event")

	if s.whitelist.IsIgnored(*event.Repo.Name) {
		logger.Info("ignored-repo", lager.Data{
			"repo": *event.Repo.Name,
		})

		s.ignoredEventCounter.Inc(logger)

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
		diffScanner := git.NewDiffScanner(diff)

		handleViolation := s.createHandleViolation(logger, currentSHA, *event.Repo.FullName, &violations)
		s.sniff(logger, diffScanner, handleViolation)

		previousSHA = currentSHA
	}

	if violations > 0 {
		s.credentialCounter.IncN(logger, violations)
	}
}

func (s *eventHandler) createHandleViolation(logger lager.Logger, sha string, repoName string, violations *int) func(sniff.Line) {
	return func(line sniff.Line) {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})

		s.notifier.SendNotification(logger, repoName, sha, line)

		*violations++
	}
}

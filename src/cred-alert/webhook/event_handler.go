package webhook

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners/git"
	"cred-alert/sniff"
	"fmt"

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

	scan, valid := Extract(logger, event)
	if !valid {
		panic("what what what")
	}

	violations := 0

	for _, scanDiff := range scan.Diffs {
		diff, err := s.githubClient.CompareRefs(logger, scan.Owner, scan.Repository, scanDiff.Start, scanDiff.End)
		if err != nil {
			logger.Error("failed-fetch-diff", err, lager.Data{
				"start": scanDiff.Start,
				"end":   scanDiff.End,
			})
			continue
		}
		diffScanner := git.NewDiffScanner(diff)

		handleViolation := s.createHandleViolation(logger, scanDiff.Start, scan.FullRepoName(), &violations)
		s.sniff(logger, diffScanner, handleViolation)

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

type PushScan struct {
	Owner      string
	Repository string

	Diffs []PushScanDiff
}

func (p PushScan) FullRepoName() string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repository)
}

type PushScanDiff struct {
	Start string
	End   string
}

func Extract(logger lager.Logger, event github.PushEvent) (PushScan, bool) {
	if len(event.Commits) == 0 {
		return PushScan{}, false
	}

	diffs := []PushScanDiff{
		{Start: *event.Before, End: *event.Commits[0].ID},
	}

	for i, _ := range event.Commits {
		if i == len(event.Commits)-1 {
			break
		}

		start := *event.Commits[i].ID
		end := *event.Commits[i+1].ID

		diffs = append(diffs, PushScanDiff{
			Start: start,
			End:   end,
		})
	}

	return PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		Diffs:      diffs,
	}, true
}

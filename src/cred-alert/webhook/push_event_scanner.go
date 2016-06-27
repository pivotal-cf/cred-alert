package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"

	myGithub "cred-alert/github"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	ScanPushEvent(lager.Logger, github.PushEvent)
}

type PushEventScanner struct {
	githubClient myGithub.Client
	scan         func(lager.Logger, string) []git.Line
	emitter      logging.Emitter
}

func NewPushEventScanner(githubClient myGithub.Client, scan func(lager.Logger, string) []git.Line, emitter logging.Emitter) *PushEventScanner {
	scanner := PushEventScanner{
		githubClient: githubClient,
		scan:         scan,
		emitter:      emitter,
	}

	return &scanner
}

func (s PushEventScanner) ScanPushEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("scan-event")

	diff, err := s.githubClient.CompareRefs(logger, *event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
	if err != nil {
		return
	}

	lines := s.scan(logger, diff)
	for _, line := range lines {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
		})
	}

	s.emitter.CountViolation(logger, len(lines))
}

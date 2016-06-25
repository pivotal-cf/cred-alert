package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"

	gh "cred-alert/github"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	ScanPushEvent(lager.Logger, github.PushEvent)
}

type PushEventScanner struct {
	fetchDiff func(lager.Logger, github.PushEvent) (string, error)
	scan      func(lager.Logger, string) []git.Line
	emitter   logging.Emitter
}

func NewPushEventScanner(fetchDiff func(lager.Logger, github.PushEvent) (string, error), scan func(lager.Logger, string) []git.Line, emitter logging.Emitter) *PushEventScanner {
	scanner := new(PushEventScanner)
	scanner.fetchDiff = fetchDiff
	scanner.scan = scan
	scanner.emitter = emitter

	return scanner
}

func DefaultPushEventScanner() *PushEventScanner {
	var scanner *PushEventScanner

	emitter, err := logging.DefaultEmitter()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: ", err)
		scanner = NewPushEventScanner(fetchDiff, git.Scan, nil)
	} else {
		scanner = NewPushEventScanner(fetchDiff, git.Scan, emitter)
	}

	return scanner
}

func (s PushEventScanner) ScanPushEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("scan-event")

	diff, err := s.fetchDiff(logger, event)
	if err != nil {
		return
	}

	lines := s.scan(logger, diff)

	for _, line := range lines {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
		})

		if s.emitter == nil {
			fmt.Fprintf(os.Stderr, "Error: data dog client is missing")
		} else {
			s.emitter.CountViolation(logger, len(lines))
		}
	}
}

func fetchDiff(logger lager.Logger, event github.PushEvent) (string, error) {
	httpClient := &http.Client{}
	githubClient := gh.NewClient("https://api.github.com/", httpClient)

	return githubClient.CompareRefs(logger, *event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
}

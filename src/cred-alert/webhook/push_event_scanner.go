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

type PushEventScanner struct {
	fetchDiff func(github.PushEvent) (string, error)
	scan      func(string) []git.Line
	emitter   logging.Emitter
}

func NewPushEventScanner(fetchDiff func(github.PushEvent) (string, error), scan func(string) []git.Line, emitter logging.Emitter) *PushEventScanner {
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
	diff, err := s.fetchDiff(event)
	if err != nil {
		logger.Error("failed-to-fetch-diff", err)
	}

	lines := s.scan(diff)

	for _, line := range lines {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
		})

		if s.emitter == nil {
			fmt.Fprintf(os.Stderr, "Error: data dog client is missing")
		} else {
			s.emitter.CountViolation(len(lines))
		}
	}
}

func fetchDiff(event github.PushEvent) (string, error) {
	httpClient := &http.Client{}
	githubClient := gh.NewClient("https://api.github.com/", httpClient)

	diff, err := githubClient.CompareRefs(*event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
	if err != nil {
		return "", err
	}

	return diff, nil
}

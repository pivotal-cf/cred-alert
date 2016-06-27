package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"
	"fmt"
	"os"

	myGithub "cred-alert/github"

	"github.com/google/go-github/github"
)

type PushEventScanner struct {
	githubClient myGithub.Client
	scan         func(string) []git.Line
	emitter      logging.Emitter
}

func NewPushEventScanner(githubClient myGithub.Client, scan func(string) []git.Line, emitter logging.Emitter) *PushEventScanner {
	scanner := PushEventScanner{
		githubClient: githubClient,
		scan:         scan,
		emitter:      emitter,
	}

	return &scanner
}

func DefaultPushEventScanner(githubClient myGithub.Client) *PushEventScanner {
	var scanner *PushEventScanner

	emitter, err := logging.DefaultEmitter()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: ", err)
		scanner = NewPushEventScanner(githubClient, git.Scan, nil)
	} else {
		scanner = NewPushEventScanner(githubClient, git.Scan, emitter)
	}

	return scanner
}

func (s PushEventScanner) ScanPushEvent(event github.PushEvent) {
	diff, err := s.githubClient.CompareRefs(*event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request error: ", err)
	}

	lines := s.scan(diff)
	for _, line := range lines {
		fmt.Printf("Found match in repo: %s, file: %s, After SHA: %s, line number: %d\n",
			*event.Repo.FullName,
			line.Path,
			*event.After,
			line.LineNumber)
	}

	if s.emitter == nil {
		fmt.Fprintf(os.Stderr, "Error: data dog client is missing")
	} else {
		s.emitter.CountViolation(len(lines))
	}
}

package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"
	"fmt"
	"os"

	"github.com/google/go-github/github"
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

func (s PushEventScanner) ScanPushEvent(event github.PushEvent) {
	diff, err := s.fetchDiff(event)
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
		if s.emitter == nil {
			fmt.Fprintf(os.Stderr, "Error: data dog client is missing")
		} else {
			s.emitter.CountViolation()
		}
	}
}

package sniff

import (
	"cred-alert/sniff/patterns"

	"github.com/pivotal-golang/lager"
)

type Line struct {
	Path       string
	LineNumber int
	Content    string

	action string
}

type Scanner interface {
	Scan(lager.Logger) bool
	Line() *Line
}

func Sniff(logger lager.Logger, scanner Scanner, handleViolation func(Line)) {
	logger = logger.Session("sniff")

	matcher := patterns.DefaultMatcher()

	for scanner.Scan(logger) {
		line := *scanner.Line()
		found := matcher.Match(line.Content)

		if found {
			handleViolation(line)
		}
	}
}

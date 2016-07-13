package sniff

import (
	"cred-alert/scanners"
	"cred-alert/sniff/patterns"

	"github.com/pivotal-golang/lager"
)

type Scanner interface {
	Scan(lager.Logger) bool
	Line() *scanners.Line
}

type SniffFunc func(lager.Logger, Scanner, func(scanners.Line))

func Sniff(logger lager.Logger, scanner Scanner, handleViolation func(scanners.Line)) {
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

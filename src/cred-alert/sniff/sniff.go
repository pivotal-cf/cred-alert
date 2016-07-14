package sniff

import (
	"cred-alert/scanners"
	"cred-alert/sniff/patterns"

	"github.com/hashicorp/go-multierror"
	"github.com/pivotal-golang/lager"
)

type Scanner interface {
	Scan(lager.Logger) bool
	Line() *scanners.Line
}

type SniffFunc func(lager.Logger, Scanner, func(scanners.Line) error) error

func Sniff(logger lager.Logger, scanner Scanner, handleViolation func(scanners.Line) error) error {
	logger = logger.Session("sniff")

	matcher := patterns.DefaultMatcher()

	var result error

	for scanner.Scan(logger) {
		line := *scanner.Line()
		found := matcher.Match(line.Content)

		if found {
			err := handleViolation(line)
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}

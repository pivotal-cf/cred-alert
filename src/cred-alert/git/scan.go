package git

import (
	"cred-alert/patterns"

	"github.com/pivotal-golang/lager"
)

func Scan(logger lager.Logger, input string) []Line {
	matcher := patterns.DefaultMatcher()
	diffScanner := NewDiffScanner(input)

	matchingLines := []Line{}

	for diffScanner.Scan() {
		line := *diffScanner.Line()
		found := matcher.Match(line.Content)

		if found {
			matchingLines = append(matchingLines, line)
		}
	}

	return matchingLines
}

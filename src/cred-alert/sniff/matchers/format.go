package matchers

import (
	"cred-alert/scanners"
	"regexp"
)

type formatMatcher struct {
	r *regexp.Regexp
}

func Format(format string) Matcher {
	return &formatMatcher{
		r: regexp.MustCompile(format),
	}
}

func (m *formatMatcher) Match(line *scanners.Line) bool {
	return m.r.Match(line.Content)
}

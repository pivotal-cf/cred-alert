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

func (m *formatMatcher) Match(line *scanners.Line) (bool, int, int) {
	index := m.r.FindIndex(line.Content)
	if index == nil {
		return false, 0, 0
	}

	return true, index[0], index[1]
}

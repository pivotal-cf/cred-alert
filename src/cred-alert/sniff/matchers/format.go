package matchers

import "regexp"

type formatMatcher struct {
	r *regexp.Regexp
}

func Format(format string) Matcher {
	return &formatMatcher{
		r: regexp.MustCompile(format),
	}
}

func (m *formatMatcher) Match(line []byte) bool {
	return m.r.Match(line)
}

package matchers

import "strings"

type substringMatcher struct {
	s string
}

func Substring(s string) Matcher {
	return &substringMatcher{
		s: s,
	}
}

func (m *substringMatcher) Match(line string) bool {
	return strings.Contains(line, m.s)
}

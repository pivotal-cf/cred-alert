package matchers

import "bytes"

type substringMatcher struct {
	s []byte
}

func Substring(s string) Matcher {
	return &substringMatcher{
		s: []byte(s),
	}
}

func (m *substringMatcher) Match(line []byte) bool {
	return bytes.Contains(line, m.s)
}

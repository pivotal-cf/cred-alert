package matchers

import (
	"bytes"
	"cred-alert/scanners"
)

type substringMatcher struct {
	s []byte
}

func Substring(s string) Matcher {
	return &substringMatcher{
		s: []byte(s),
	}
}

func (m *substringMatcher) Match(line *scanners.Line) bool {
	return bytes.Contains(line.Content, m.s)
}

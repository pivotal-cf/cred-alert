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

func (m *substringMatcher) Match(line []byte) (bool, int, int) {
	start := bytes.Index(line, m.s)
	if start == -1 {
		return false, 0, 0
	}

	end := start + len(m.s)

	return true, start, end
}

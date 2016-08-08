package matchers

import "bytes"

func UpcasedMulti(matchers ...Matcher) Matcher {
	return &multi{
		matchers: matchers,
	}
}

type multi struct {
	matchers []Matcher
}

func (m *multi) Match(line []byte) bool {
	upcasedLine := bytes.ToUpper(line)
	for _, matcher := range m.matchers {
		if matcher.Match(upcasedLine) {
			return true
		}
	}

	return false
}

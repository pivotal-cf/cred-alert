package matchers

import "strings"

func UpcasedMulti(matchers ...Matcher) Matcher {
	return &multi{
		matchers: matchers,
	}
}

type multi struct {
	matchers []Matcher
}

func (m *multi) Match(line string) bool {
	upcasedLine := strings.ToUpper(line)
	for _, matcher := range m.matchers {
		if matcher.Match(upcasedLine) {
			return true
		}
	}

	return false
}

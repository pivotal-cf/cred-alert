package matchers

import (
	"bytes"
	"cred-alert/scanners"
)

func UpcasedMulti(matchers ...Matcher) Matcher {
	return &multi{
		matchers: matchers,
	}
}

type multi struct {
	matchers []Matcher
}

func (m *multi) Match(line *scanners.Line) bool {
	upcasedLine := &scanners.Line{
		Content:    bytes.ToUpper(line.Content),
		Path:       line.Path,
		LineNumber: line.LineNumber,
	}

	for _, matcher := range m.matchers {
		if matcher.Match(upcasedLine) {
			return true
		}
	}

	return false
}

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

func (m *multi) Match(line *scanners.Line) (bool, int, int) {
	upcasedLine := &scanners.Line{
		Content:    bytes.ToUpper(line.Content),
		Path:       line.Path,
		LineNumber: line.LineNumber,
	}

	for _, matcher := range m.matchers {
		if match, start, end := matcher.Match(upcasedLine); match {
			return true, start, end
		}
	}

	return false, 0, 0
}

package matchers

import (
	"bufio"
	"bytes"
	"cred-alert/scanners"
	"io"
	"strings"
)

func UpcasedMulti(matchers ...Matcher) Matcher {
	return &multi{
		matchers: matchers,
	}
}

func UpcasedMultiMatcherFromReader(rd io.Reader) Matcher {
	scanner := bufio.NewScanner(rd)

	var matchers []Matcher
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		matchers = append(matchers, Format(strings.ToUpper(line)))
	}

	return UpcasedMulti(matchers...)
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

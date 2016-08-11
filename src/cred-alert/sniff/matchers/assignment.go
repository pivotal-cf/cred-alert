package matchers

import (
	"bytes"
	"regexp"
)

const generalPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT)["']?\s*(=|:|:=|=>)?\s*["']?[A-Z0-9.$+=&\/_\\-]{12,}`
const assignmentPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT)["']?\s*(?:=|:|:=|=>)?\s*["'][A-Z0-9.$+=&\/_\\-]{12,}["']`
const yamlPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT):\s*["']?[A-Z0-9.$+=&\/_\\-]{12,}`
const guidPattern = `[A-F0-9]{8}-[A-F0-9]{4}-[1-5][A-F0-9]{3}-[A-F0-9]{4}-[A-F0-9]{12}`

func Assignment() Matcher {
	return &assignmentMatcher{
		pattern:           regexp.MustCompile(generalPattern),
		assignmentPattern: regexp.MustCompile(assignmentPattern),
		yamlPattern:       regexp.MustCompile(yamlPattern),
		guidPattern:       regexp.MustCompile(guidPattern),
	}
}

type assignmentMatcher struct {
	pattern           *regexp.Regexp
	assignmentPattern *regexp.Regexp
	yamlPattern       *regexp.Regexp
	guidPattern       *regexp.Regexp
}

func (m *assignmentMatcher) Match(line []byte) bool {
	upcasedLine := bytes.ToUpper(line)
	matchIndexPairs := m.pattern.FindSubmatchIndex(upcasedLine)
	if matchIndexPairs == nil {
		return false
	}

	if m.guidPattern.Match(upcasedLine) {
		return false
	}

	if isYAMLAssignment(matchIndexPairs, line) {
		return m.yamlPattern.Match(upcasedLine)
	}

	return m.assignmentPattern.Match(upcasedLine)
}

func isYAMLAssignment(matchIndexPairs []int, line []byte) bool {
	startMatch := matchIndexPairs[2]
	endMatch := matchIndexPairs[3]
	if startMatch != -1 && endMatch != 1 {
		return bytes.Compare(line[startMatch:endMatch], []byte(":")) == 0
	}

	return false
}

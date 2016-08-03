package matchers

import (
	"regexp"
	"strings"
	"sync"
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
		l:                 &sync.Mutex{},
	}
}

type assignmentMatcher struct {
	pattern           *regexp.Regexp
	assignmentPattern *regexp.Regexp
	yamlPattern       *regexp.Regexp
	guidPattern       *regexp.Regexp
	l                 *sync.Mutex
}

func (m *assignmentMatcher) Match(line string) bool {
	m.l.Lock()
	defer m.l.Unlock()

	upcasedLine := strings.ToUpper(line)
	result := m.pattern.FindStringSubmatch(upcasedLine)
	if result == nil {
		return false
	}

	if m.guidPattern.MatchString(upcasedLine) {
		return false
	}

	if isYAMLAssignment(result) {
		return m.yamlPattern.MatchString(upcasedLine)
	}

	return m.assignmentPattern.MatchString(upcasedLine)
}

func isYAMLAssignment(result []string) bool {
	return result[1] == ":"
}

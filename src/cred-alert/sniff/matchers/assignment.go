package matchers

import (
	"regexp"
	"strings"
)

const generalPattern = `(?i)["']?[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']?\s*(=|:|:=|=>)?\s*["']?[A-Za-z0-9.$+=&\/_\\-]{12,}["']?`

const assignmentPattern = `(?i)["']?[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']?\s*(=|:|:=|=>)?\s*["'][A-Za-z0-9.$+=&\/_\\-]{12,}["']`

const yamlPattern = `(?i)[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt):\s*["']?[A-Za-z0-9.$+=&\/_\\-]{12,}["']?`

func Assignment() Matcher {
	return &assignmentMatcher{
		pattern:           regexp.MustCompile(generalPattern),
		assignmentPattern: regexp.MustCompile(assignmentPattern),
		yamlPattern:       regexp.MustCompile(yamlPattern),
	}
}

type assignmentMatcher struct {
	pattern           *regexp.Regexp
	assignmentPattern *regexp.Regexp
	yamlPattern       *regexp.Regexp
}

func (m *assignmentMatcher) Match(line string) bool {
	if !m.pattern.MatchString(line) {
		return false
	}

	if isYAML(line) {
		return m.yamlPattern.MatchString(line)
	}

	return m.assignmentPattern.MatchString(line)
}

func isYAML(line string) bool {
	idx := strings.IndexAny(line, ":")
	return idx != -1 && line[idx+1] != '='
}

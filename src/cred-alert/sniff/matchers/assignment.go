package matchers

import "regexp"

const generalPattern = `(?i)["']?[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']?\s*(=|:|:=|=>)?\s*["']?[A-Za-z0-9.$+=&\/_\\-]{12,}["']?`

const assignmentPattern = `(?i)["']?[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']?\s*(=|:|:=|=>)?\s*["'][A-Za-z0-9.$+=&\/_\\-]{12,}["']`

const yamlPattern = `(?i)[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt):\s*["']?[A-Za-z0-9.$+=&\/_\\-]{12,}["']?`

const guidPattern = `(?i)[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[a-f0-9]{4}-[a-f0-9]{12}`

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

func (m *assignmentMatcher) Match(line string) bool {
	result := m.pattern.FindStringSubmatch(line)
	if result == nil {
		return false
	}

	if m.guidPattern.MatchString(line) {
		return false
	}

	if isYAMLAssignment(result) {
		return m.yamlPattern.MatchString(line)
	}

	return m.assignmentPattern.MatchString(line)
}

func isYAMLAssignment(result []string) bool {
	return result[2] == ":"
}

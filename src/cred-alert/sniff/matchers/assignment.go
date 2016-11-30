package matchers

import (
	"cred-alert/scanners"
	"path/filepath"
	"regexp"
)

const assignmentPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT)\s*(?::=|=>|=|:|\s)\s*([\w\S-]+)`
const nonYamlAssignmentPattern = `["'].{12,}["']`
const yamlAssignmentPattern = `["']?[\w\-\(\){}]{12,}["']?`

const guidPattern = `[A-F0-9]{8}-[A-F0-9]{4}-[1-5][A-F0-9]{3}-[A-F0-9]{4}-[A-F0-9]{12}`
const placeholderPattern = `(?:\(\(|\{\{)\s*[\w/.-]+\s*(?:\)\)|\}\})`

func Assignment() Matcher {
	return &assignmentMatcher{
		assignmentPattern:        regexp.MustCompile(assignmentPattern),
		nonYamlAssignmentPattern: regexp.MustCompile(nonYamlAssignmentPattern),
		yamlAssignmentPattern:    regexp.MustCompile(yamlAssignmentPattern),

		guidPattern:        regexp.MustCompile(guidPattern),
		placeholderPattern: regexp.MustCompile(placeholderPattern),
	}
}

type assignmentMatcher struct {
	assignmentPattern        *regexp.Regexp
	nonYamlAssignmentPattern *regexp.Regexp
	yamlAssignmentPattern    *regexp.Regexp
	guidPattern              *regexp.Regexp
	placeholderPattern       *regexp.Regexp
}

func (m *assignmentMatcher) Match(line *scanners.Line) (bool, int, int) {
	matchIndexPairs := m.assignmentPattern.FindSubmatchIndex(line.Content)
	if matchIndexPairs == nil {
		return false, 0, 0
	}

	content := line.Content[matchIndexPairs[2]:matchIndexPairs[3]]

	if m.guidPattern.Match(content) {
		return false, 0, 0
	}

	ext := filepath.Ext(line.Path)
	if ext == ".yml" || ext == ".yaml" {
		if m.placeholderPattern.Match(content) {
			return false, 0, 0
		}

		if m.yamlAssignmentPattern.Match(content) {
			return true, matchIndexPairs[0], matchIndexPairs[1]
		}
	}

	if m.nonYamlAssignmentPattern.Match(line.Content) {
		return true, matchIndexPairs[0], matchIndexPairs[1]
	}

	return false, 0, 0
}

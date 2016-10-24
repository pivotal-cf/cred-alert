package matchers

import (
	"cred-alert/scanners"
	"path/filepath"
	"regexp"
)

const assignmentPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT)["']?\s*(?:=|:|:=|=>)?\s*(["'])?[A-Z0-9.$+=&\\\/_\-\(\){} ]{12,}(["'])?`
const yamlPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT):\s*["']?[A-Z0-9.$+=&\\\/_\-\(\){} ]{12,}`
const guidPattern = `[A-F0-9]{8}-[A-F0-9]{4}-[1-5][A-F0-9]{3}-[A-F0-9]{4}-[A-F0-9]{12}`
const placeholderPattern = `(\(\(|\{\{)[ \t]*[A-Z0-9_.-]+[ \t]*(\)\)|\}\})`

func Assignment() Matcher {
	return &assignmentMatcher{
		assignmentPattern:  regexp.MustCompile(assignmentPattern),
		yamlPattern:        regexp.MustCompile(yamlPattern),
		guidPattern:        regexp.MustCompile(guidPattern),
		placeholderPattern: regexp.MustCompile(placeholderPattern),
	}
}

type assignmentMatcher struct {
	assignmentPattern  *regexp.Regexp
	yamlPattern        *regexp.Regexp
	guidPattern        *regexp.Regexp
	placeholderPattern *regexp.Regexp
}

func (m *assignmentMatcher) Match(line *scanners.Line) (bool, int, int) {
	matchIndexPairs := m.assignmentPattern.FindSubmatchIndex(line.Content)
	if matchIndexPairs == nil {
		return false, 0, 0
	}

	content := line.Content[matchIndexPairs[0]:matchIndexPairs[1]]
	if m.guidPattern.Match(content) {
		return false, 0, 0
	}

	ext := filepath.Ext(line.Path)
	if ext == ".yml" || ext == ".yaml" {
		if m.placeholderPattern.Match(content) {
			return false, 0, 0
		}

		return m.yamlPattern.Match(content), matchIndexPairs[0], matchIndexPairs[1]
	}

	quoteExists := matchIndexPairs[3] != -1
	return quoteExists, matchIndexPairs[0], matchIndexPairs[1]
}

package matchers

import (
	"cred-alert/scanners"
	"path/filepath"
	"regexp"
)

const assignmentPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT)["']?\s*(?:=|:|:=|=>)?\s*(["'])?[A-Z0-9.$+=&\/_\\-]{12,}(["'])?`
const yamlPattern = `(?:SECRET|PRIVATE[-_]?KEY|PASSWORD|SALT):\s*["']?[A-Z0-9.$+=&\/_\\-]{12,}`
const guidPattern = `[A-F0-9]{8}-[A-F0-9]{4}-[1-5][A-F0-9]{3}-[A-F0-9]{4}-[A-F0-9]{12}`

func Assignment() Matcher {
	return &assignmentMatcher{
		assignmentPattern: regexp.MustCompile(assignmentPattern),
		yamlPattern:       regexp.MustCompile(yamlPattern),
		guidPattern:       regexp.MustCompile(guidPattern),
	}
}

type assignmentMatcher struct {
	assignmentPattern *regexp.Regexp
	yamlPattern       *regexp.Regexp
	guidPattern       *regexp.Regexp
}

func (m *assignmentMatcher) Match(line *scanners.Line) bool {
	matchIndexPairs := m.assignmentPattern.FindSubmatchIndex(line.Content)
	if matchIndexPairs == nil {
		return false
	}

	content := line.Content[matchIndexPairs[0]:matchIndexPairs[1]]
	if m.guidPattern.Match(content) {
		return false
	}

	ext := filepath.Ext(line.Path)
	if ext == ".yml" || ext == ".yaml" {
		return m.yamlPattern.Match(content)
	}

	quoteExists := matchIndexPairs[3] != -1
	return quoteExists
}

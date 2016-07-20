package patterns

import (
	"regexp"
)

var myRegex = regexp.MustCompile(`password`)

type Matcher interface {
	Match(string) bool
}

type matcher struct {
	patterns   []*regexp.Regexp
	exclusions []*regexp.Regexp
}

func NewMatcher(patterns []string, exclusions []string) Matcher {
	m := new(matcher)

	m.patterns = make([]*regexp.Regexp, len(patterns))
	m.exclusions = make([]*regexp.Regexp, len(exclusions))

	for i, pattern := range patterns {
		m.patterns[i] = regexp.MustCompile(pattern)
	}

	for i, exclusion := range exclusions {
		m.exclusions[i] = regexp.MustCompile(exclusion)
	}

	return m
}

func (m *matcher) Match(to_search string) bool {
	for _, exclusion := range m.exclusions {
		excluded := exclusion.MatchString(to_search)
		if excluded {
			return false
		}
	}

	for _, pattern := range m.patterns {
		found := pattern.MatchString(to_search)
		if found {
			return true
		}
	}

	return false
}

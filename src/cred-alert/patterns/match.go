package patterns

import (
	"regexp"
)

var myRegex = regexp.MustCompile(`password`)

type Matcher struct {
	patterns   []*regexp.Regexp
	exclusions []*regexp.Regexp
}

func NewMatcher(patterns []string, exclusions []string) *Matcher {
	matcher := new(Matcher)

	matcher.patterns = make([]*regexp.Regexp, len(patterns))
	matcher.exclusions = make([]*regexp.Regexp, len(exclusions))

	for i, pattern := range patterns {
		matcher.patterns[i] = regexp.MustCompile(pattern)
	}

	for i, exclusion := range exclusions {
		matcher.exclusions[i] = regexp.MustCompile(exclusion)
	}

	return matcher
}

func (m Matcher) Match(to_search string) bool {
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

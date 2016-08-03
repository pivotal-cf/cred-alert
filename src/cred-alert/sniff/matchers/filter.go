package matchers

import "strings"

func Filter(submatcher Matcher, filters ...string) Matcher {
	return &filter{
		matcher: submatcher,
		filters: filters,
	}
}

type filter struct {
	matcher Matcher
	filters []string
}

func (f *filter) Match(line string) bool {
	found := false

	for i := range f.filters {
		if strings.Contains(line, f.filters[i]) {
			found = true
			break
		}
	}

	if !found {
		return false
	}

	return f.matcher.Match(line)
}

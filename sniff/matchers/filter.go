package matchers

import "bytes"

func Filter(submatcher Matcher, filters ...string) Matcher {
	fs := make([][]byte, len(filters))

	for i := range filters {
		fs[i] = []byte(filters[i])
	}

	return &filter{
		matcher: submatcher,
		filters: fs,
	}
}

type filter struct {
	matcher Matcher
	filters [][]byte
}

func (f *filter) Match(line []byte) (bool, int, int) {
	found := false

	for i := range f.filters {
		if bytes.Contains(line, f.filters[i]) {
			found = true
			break
		}
	}

	if !found {
		return false, 0, 0
	}

	return f.matcher.Match(line)
}

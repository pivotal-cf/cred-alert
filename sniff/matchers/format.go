package matchers

import "regexp"

type formatMatcher struct {
	r *regexp.Regexp
}

func Format(format string) Matcher {
	return &formatMatcher{
		r: regexp.MustCompile(format),
	}
}

func TryFormat(format string) (Matcher, error) {
	r, err := regexp.Compile(format)
	if err != nil {
		return nil, err
	}

	return &formatMatcher{
		r: r,
	}, nil
}

func (m *formatMatcher) Match(line []byte) (bool, int, int) {
	index := m.r.FindIndex(line)
	if index == nil {
		return false, 0, 0
	}

	return true, index[0], index[1]
}

package matchers

import "regexp"

type knownFormat struct {
	r *regexp.Regexp
}

func KnownFormat(format string) Matcher {
	return &knownFormat{
		r: regexp.MustCompile(format),
	}
}

func (m *knownFormat) Match(line string) bool {
	return m.r.MatchString(line)
}

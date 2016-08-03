package matchers

import (
	"regexp"
	"sync"
)

type formatMatcher struct {
	r *regexp.Regexp
	l *sync.Mutex
}

func Format(format string) Matcher {
	return &formatMatcher{
		r: regexp.MustCompile(format),
		l: &sync.Mutex{},
	}
}

func (m *formatMatcher) Match(line string) bool {
	m.l.Lock()
	defer m.l.Unlock()

	return m.r.MatchString(line)
}

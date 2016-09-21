package notifications

import "regexp"

type Whitelist interface {
	ShouldSkipNotification(bool, string) bool
}

func BuildWhitelist(regexs ...string) *whitelist {
	patterns := make([]*regexp.Regexp, len(regexs))

	for i, uncompiled := range regexs {
		patterns[i] = regexp.MustCompile("^" + uncompiled + "$")
	}

	return &whitelist{
		patterns: patterns,
	}
}

type whitelist struct {
	patterns []*regexp.Regexp
}

func (w *whitelist) ShouldSkipNotification(isPrivate bool, name string) bool {
	return isPrivate && w.nameMatchesPattern(name)
}

func (w *whitelist) nameMatchesPattern(name string) bool {
	for _, pattern := range w.patterns {
		if pattern.MatchString(name) {
			return true
		}
	}

	return false
}

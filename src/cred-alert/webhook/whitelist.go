package webhook

import "regexp"

func BuildWhitelist(regexs ...string) *Whitelist {
	patterns := make([]*regexp.Regexp, len(regexs))

	for i, uncompiled := range regexs {
		patterns[i] = regexp.MustCompile("^" + uncompiled + "$")
	}

	return &Whitelist{
		patterns: patterns,
	}
}

type Whitelist struct {
	patterns []*regexp.Regexp
}

func (w *Whitelist) IsIgnored(name string) bool {
	for _, pattern := range w.patterns {
		if pattern.MatchString(name) {
			return true
		}
	}

	return false
}

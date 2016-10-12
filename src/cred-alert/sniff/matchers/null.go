package matchers

import "cred-alert/scanners"

type nullMatcher struct{}

func NewNullMatcher() Matcher {
	return &nullMatcher{}
}

func (n *nullMatcher) Match(*scanners.Line) (bool, int, int) {
	return false, 0, 0
}

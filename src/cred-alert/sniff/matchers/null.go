package matchers

type nullMatcher struct{}

func NewNullMatcher() Matcher {
	return &nullMatcher{}
}

func (n *nullMatcher) Match([]byte) (bool, int, int) {
	return false, 0, 0
}

package matchers

func Multi(matchers ...Matcher) Matcher {
	return &multi{
		matchers: matchers,
	}
}

type multi struct {
	matchers []Matcher
}

func (m *multi) Match(line string) bool {
	for _, matcher := range m.matchers {
		if matcher.Match(line) {
			return true
		}
	}

	return false
}

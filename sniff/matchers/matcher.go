package matchers

//go:generate counterfeiter . Matcher

type Matcher interface {
	Match([]byte) (bool, int, int)
}

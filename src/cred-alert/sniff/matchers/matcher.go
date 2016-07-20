package matchers

//go:generate counterfeiter . Matcher

type Matcher interface {
	Match(string) bool
}

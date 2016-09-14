package matchers

import "cred-alert/scanners"

//go:generate counterfeiter . Matcher

type Matcher interface {
	Match(*scanners.Line) (bool, int, int)
}

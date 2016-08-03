package sniff

import (
	"cred-alert/scanners"
	"cred-alert/sniff/matchers"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
)

const bashStringInterpolationPattern = `"$`
const fakePattern = `FAKE`
const examplePattern = `EXAMPLE`

const awsAccessKeyIDPattern = `AKIA[A-Z0-9]{16}`
const awsSecretAccessKeyPattern = `KEY["']?\s*(?::|=>|=)\s*["']?[A-Z0-9/\+=]{40}["']?`
const cryptMD5Pattern = `\$1\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{22}`
const cryptSHA256Pattern = `\$5\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{43}`
const cryptSHA512Pattern = `\$6\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{86}`
const rsaPrivateKeyHeaderPattern = `-----BEGIN RSA PRIVATE KEY-----`

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger) bool
	Line(lager.Logger) *scanners.Line
}

//go:generate counterfeiter . Sniffer

type Sniffer interface {
	Sniff(lager.Logger, Scanner, func(lager.Logger, scanners.Line) error) error
}

type sniffer struct {
	matcher          matchers.Matcher
	exclusionMatcher matchers.Matcher
}

func NewSniffer(matcher, exclusionMatcher matchers.Matcher) Sniffer {
	return &sniffer{
		matcher:          matcher,
		exclusionMatcher: exclusionMatcher,
	}
}

func NewDefaultSniffer() Sniffer {
	return &sniffer{
		matcher: matchers.UpcasedMulti(
			matchers.Filter(matchers.Format(awsAccessKeyIDPattern), "AKIA"),
			matchers.Format(awsSecretAccessKeyPattern),
			matchers.Filter(matchers.Format(cryptMD5Pattern), "$1$"),
			matchers.Filter(matchers.Format(cryptSHA256Pattern), "$5$"),
			matchers.Filter(matchers.Format(cryptSHA512Pattern), "$6$"),
			matchers.Substring(rsaPrivateKeyHeaderPattern),
			matchers.Filter(matchers.Assignment(), "=", ":=", ":", "=>", "SECRET", "PRIVATE", "KEY", "PASSWORD", "SALT"),
		),
		exclusionMatcher: matchers.UpcasedMulti(
			matchers.Substring(bashStringInterpolationPattern),
			matchers.Substring(fakePattern),
			matchers.Substring(examplePattern),
		),
	}
}

func (s *sniffer) Sniff(
	logger lager.Logger,
	scanner Scanner,
	handleViolation func(lager.Logger, scanners.Line) error,
) error {
	logger = logger.Session("sniff")
	logger.Debug("starting")

	var result error

	for scanner.Scan(logger) {
		line := *scanner.Line(logger)

		if s.exclusionMatcher.Match(line.Content) {
			continue
		}

		if s.matcher.Match(line.Content) {
			err := handleViolation(logger, line)
			if err != nil {
				logger.Error("failed", err)
				result = multierror.Append(result, err)
			}
		}
	}

	logger.Debug("done")
	return result
}

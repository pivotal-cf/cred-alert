package sniff

import (
	"fmt"
	"regexp"

	"github.com/pivotal-cf/cred-alert/scanners"
	"github.com/pivotal-cf/cred-alert/sniff/matchers"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
)

const bashStringInterpolationPattern = `"$`
const fakePattern = `FAKE`
const changePattern = `CHANGE`
const replacePattern = `REPLACE`
const examplePattern = `EXAMPLE`

const awsAccessKeyIDPattern = `(^|[^A-Z0-9])AKIA[A-Z0-9]{16}`
const awsSecretAccessKeyPattern = `KEY["']?\s*(?::|=>|=)\s*["']?[A-Z0-9/\+=]{40}["']?`
const cryptMD5Pattern = `\$1\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{22}`
const cryptSHA256Pattern = `\$5\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{43}`
const cryptSHA512Pattern = `\$6\$[A-Z0-9./]{1,16}\$[A-Z0-9./]{86}`
const privateKeyHeaderPattern = `-----BEGIN(.*)PRIVATE KEY-----`

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger) bool
	Line(lager.Logger) *scanners.Line
	Err() error
}

//go:generate counterfeiter . Sniffer

type Sniffer interface {
	Sniff(lager.Logger, Scanner, ViolationHandlerFunc) error
}

type ViolationHandlerFunc func(lager.Logger, scanners.Violation) error

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
			matchers.Format(privateKeyHeaderPattern),
		),
		exclusionMatcher: matchers.UpcasedMulti(
			matchers.Substring(bashStringInterpolationPattern),
			matchers.Substring(fakePattern),
			matchers.Substring(examplePattern),
			matchers.Substring(changePattern),
			matchers.Substring(replacePattern),
		),
	}
}

func (s *sniffer) Sniff(
	logger lager.Logger,
	scanner Scanner,
	handleViolation ViolationHandlerFunc,
) error {
	logger = logger.Session("sniff")
	logger.Debug("starting")

	var result error

	for scanner.Scan(logger) {
		line := scanner.Line(logger)

		vendorRe := regexp.MustCompile(`\/?vendor/`)
		if vendorRe.Match([]byte(line.Path)) {
			continue
		}

		if s.exclusionMatcher != nil {
			if match, _, _ := s.exclusionMatcher.Match(line.Content); match {
				continue
			}
		}

		if match, start, end := s.matcher.Match(line.Content); match {
			violation := scanners.Violation{
				Line:  *line,
				Start: start,
				End:   end,
			}

			err := handleViolation(logger, violation)
			if err != nil {
				logger.Error("failed", err)
				result = multierror.Append(result, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("\033[31m[FAILED]\033[0m scanning failed: %s\n", err)
		result = multierror.Append(result, err)
	}

	logger.Debug("done")
	return result
}

package sniff

import (
	"cred-alert/scanners"
	"cred-alert/sniff/matchers"

	"github.com/hashicorp/go-multierror"
	"code.cloudfoundry.org/lager"
)

const bashStringInterpolationPattern = `["]\$`
const fakePattern = `(?i)fake`
const examplePattern = `(?i)example`

const awsAccessKeyIDPattern = `AKIA[A-Z0-9]{16}`
const awsSecretAccessKeyPattern = `(?i)("|')?(aws)?_?(secret)?_?(access)?_?(key)("|')?\s*(:|=>|=)\s*("|')?[A-Za-z0-9/\+=]{40}("|')?`
const awsAccountIDPattern = `(?i)("|')?(aws)?_?(account)_?(id)?("|')?\s*(:|=>|=)\s*("|')?[0-9]{4}\-?[0-9]{4}\-?[0-9]{4}("|')?`
const cryptMD5Pattern = `\$1\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{22}`
const cryptSHA256Pattern = `\$5\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{43}`
const cryptSHA512Pattern = `\$6\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{86}`
const rsaPrivateKeyHeaderPattern = `-----BEGIN RSA PRIVATE KEY-----`

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger) bool
	Line(lager.Logger) *scanners.Line
}

//go:generate counterfeiter . Sniffer

type Sniffer interface {
	Sniff(lager.Logger, Scanner, func(scanners.Line) error) error
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
		matcher: matchers.Multi(
			matchers.KnownFormat(awsAccessKeyIDPattern),
			matchers.KnownFormat(awsSecretAccessKeyPattern),
			matchers.KnownFormat(awsAccountIDPattern),
			matchers.KnownFormat(cryptMD5Pattern),
			matchers.KnownFormat(cryptSHA256Pattern),
			matchers.KnownFormat(cryptSHA512Pattern),
			matchers.KnownFormat(rsaPrivateKeyHeaderPattern),
			matchers.Assignment(),
		),
		exclusionMatcher: matchers.Multi(
			matchers.KnownFormat(bashStringInterpolationPattern),
			matchers.KnownFormat(fakePattern),
			matchers.KnownFormat(examplePattern),
		),
	}
}

func (s *sniffer) Sniff(
	logger lager.Logger,
	scanner Scanner,
	handleViolation func(scanners.Line) error,
) error {
	logger = logger.Session("sniff")
	logger.Info("starting")

	var result error

	for scanner.Scan(logger) {
		line := *scanner.Line(logger)

		if s.exclusionMatcher.Match(line.Content) {
			continue
		}

		if s.matcher.Match(line.Content) {
			err := handleViolation(line)
			if err != nil {
				logger.Session("handle-violation").Error("failed", err)
				result = multierror.Append(result, err)
			}
		}
	}

	logger.Info("done")
	return result
}

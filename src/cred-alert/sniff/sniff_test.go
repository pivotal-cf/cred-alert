package sniff_test

import (
	"errors"
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/fixtures"
	"cred-alert/sniff/matchers/matchersfakes"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("Sniffer", func() {
	var (
		logger            *lagertest.TestLogger
		matcher           *matchersfakes.FakeMatcher
		exclusionMatcher  *matchersfakes.FakeMatcher
		scanner           *snifffakes.FakeScanner
		expectedLine      *scanners.Line
		expectedViolation scanners.Violation

		sniffer sniff.Sniffer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("scanner")
		matcher = new(matchersfakes.FakeMatcher)
		exclusionMatcher = new(matchersfakes.FakeMatcher)
		sniffer = sniff.NewSniffer(matcher, exclusionMatcher)

		scanner = new(snifffakes.FakeScanner)
		scanner.ScanStub = func(lager.Logger) bool {
			return scanner.ScanCallCount() < 4
		}

		expectedLine = &scanners.Line{
			Path:       "some-path",
			LineNumber: 42,
			Content:    []byte("some-content"),
		}
		scanner.LineReturns(expectedLine)

		expectedViolation = scanners.Violation{
			Line:  *expectedLine,
			Start: 8,
			End:   23,
		}
	})

	Describe("Sniff", func() {
		It("calls the exclusion matcher with each line", func() {
			sniffer.Sniff(logger, scanner, func(lager.Logger, scanners.Violation) error {
				return nil
			})
			Eventually(scanner.ScanCallCount()).Should(Equal(4))
			Consistently(exclusionMatcher.MatchCallCount()).Should(Equal(3))
		})

		Context("when error occurred during scanning", func() {
			BeforeEach(func() {
				scanner.ErrReturns(errors.New("my awesome error"))
			})

			It("returns the scan error", func() {

				err := sniffer.Sniff(logger, scanner, func(lager.Logger, scanners.Violation) error {
					return nil
				})
				Eventually(scanner.ScanCallCount()).Should(Equal(4))
				Consistently(exclusionMatcher.MatchCallCount()).Should(Equal(3))
				expErr := multierror.Append(nil, errors.New("my awesome error"))
				Expect(err).To(MatchError(expErr))
			})
		})

		Context("when changes are in a directory named `vendor`", func() {
			BeforeEach(func() {
				expectedLine = &scanners.Line{
					Path:       "/test/vendor/somepath",
					LineNumber: 42,
					Content:    []byte("some-content"),
				}

				scanner.LineStub = func(logger lager.Logger) *scanners.Line {
					if scanner.LineCallCount()%2 == 0 {
						return &scanners.Line{
							Path:       "/test/vendor/somepath",
							LineNumber: 42,
							Content:    []byte("some-content"),
						}
					}
					return &scanners.Line{
						Path:       "vendor/somepath",
						LineNumber: 42,
						Content:    []byte("some-content"),
					}
				}
			})

			It("ignores all files in a directory named `vendor`", func() {
				sniffer.Sniff(logger, scanner, func(lager.Logger, scanners.Violation) error {
					return nil
				})
				Eventually(scanner.ScanCallCount()).Should(Equal(4))
				Consistently(exclusionMatcher.MatchCallCount()).Should(Equal(0))
			})
		})

		It("calls the regular matcher with each line", func() {
			sniffer.Sniff(logger, scanner, func(lager.Logger, scanners.Violation) error {
				return nil
			})
			Expect(matcher.MatchCallCount()).To(Equal(3))
		})

		Context("when the exclusion matcher returns true", func() {
			BeforeEach(func() {
				exclusionMatcher.MatchReturns(true, 7, 19)
			})

			It("does not call the regular matcher", func() {
				sniffer.Sniff(logger, scanner, func(lager.Logger, scanners.Violation) error {
					return nil
				})
				Expect(matcher.MatchCallCount()).To(BeZero())
			})
		})

		Context("when the regular matcher returns true", func() {
			BeforeEach(func() {
				matcher.MatchStub = func([]byte) (bool, int, int) {
					return matcher.MatchCallCount() != 1, 8, 23 // 2 should match
				}
			})

			It("calls the callback with the line", func() {
				var actualViolation scanners.Violation

				callback := func(logger lager.Logger, violation scanners.Violation) error {
					actualViolation = violation
					return nil
				}

				sniffer.Sniff(logger, scanner, callback)

				Expect(actualViolation).To(Equal(expectedViolation))
			})

			Context("when the callback returns an error", func() {
				var (
					callCount int
					callback  func(lager.Logger, scanners.Violation) error
				)

				BeforeEach(func() {
					callCount = 0

					callback = func(logger lager.Logger, line scanners.Violation) error {
						callCount++
						return errors.New("tragedy")
					}
				})

				It("returns an error", func() {
					err := sniffer.Sniff(logger, scanner, callback)
					Expect(err).To(HaveOccurred())
				})

				It("calls the exclusion matcher with each line", func() {
					sniffer.Sniff(logger, scanner, callback)
					Expect(exclusionMatcher.MatchCallCount()).To(Equal(3))
				})

				It("calls the regular matcher with each line", func() {
					sniffer.Sniff(logger, scanner, callback)
					Expect(matcher.MatchCallCount()).To(Equal(3))
				})

				It("calls the callback for each line that matches", func() {
					sniffer.Sniff(logger, scanner, callback)
					Expect(callCount).To(Equal(2))
				})
			})
		})
	})

	Describe("DefaultSniffer", func() {
		var lines []string
		var sniffer sniff.Sniffer

		BeforeEach(func() {
			lines = strings.Split(fixtures.Credentials, "\n")
			sniffer = sniff.NewDefaultSniffer()
		})

		It("matches all positive examples", func() {
			var expectations []string
			var actuals []string

			for _, line := range lines {
				scanner.ScanReturns(true)

				if strings.Contains(line, "should_match") {
					expectations = append(expectations, line)
				}

				scanner.LineStub = func(logger lager.Logger) *scanners.Line {
					scanner.ScanReturns(false)

					return &scanners.Line{
						Content: []byte(line),
					}
				}

				sniffer.Sniff(logger, scanner, func(logger lager.Logger, violation scanners.Violation) error {
					actuals = append(actuals, string(violation.Line.Content))
					return nil
				})
			}

			for _, actual := range actuals {
				Expect(expectations).To(ContainElement(actual))
			}

			Expect(actuals).To(HaveLen(len(expectations)))
		})

		Context("false positives", func() {
			It("should not match strings that contain the lowercase amazon key prefix", func() {
				testString := "XakiaXXXXXXXXXXXXXXXXX"
				scanner.ScanStub = func(logger lager.Logger) bool {
					if scanner.ScanCallCount() == 1 {
						return true
					}
					return false
				}

				scanner.LineStub = func(logger lager.Logger) *scanners.Line {
					return &scanners.Line{Content: []byte(testString)}
				}

				err := sniffer.Sniff(logger, scanner, func(logger lager.Logger, violation scanners.Violation) error {
					Fail(fmt.Sprintf("%s should not be a violation", testString))
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

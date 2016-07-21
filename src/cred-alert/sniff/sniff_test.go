package sniff_test

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/fixtures"
	"cred-alert/sniff/matchers/matchersfakes"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("Sniffer", func() {
	var (
		logger           *lagertest.TestLogger
		matcher          *matchersfakes.FakeMatcher
		exclusionMatcher *matchersfakes.FakeMatcher
		scanner          *snifffakes.FakeScanner
		expectedLine     *scanners.Line

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
			Content:    "some-content",
		}
		scanner.LineReturns(expectedLine)
	})

	Describe("Sniff", func() {
		It("calls the exclusion matcher with each line", func() {
			sniffer.Sniff(logger, scanner, func(scanners.Line) error {
				return nil
			})
			Expect(exclusionMatcher.MatchCallCount()).To(Equal(3))
		})

		It("calls the regular matcher with each line", func() {
			sniffer.Sniff(logger, scanner, func(scanners.Line) error {
				return nil
			})
			Expect(matcher.MatchCallCount()).To(Equal(3))
		})

		Context("when the exclusion matcher returns true", func() {
			BeforeEach(func() {
				exclusionMatcher.MatchReturns(true)
			})

			It("does not call the regular matcher", func() {
				sniffer.Sniff(logger, scanner, func(scanners.Line) error {
					return nil
				})
				Expect(matcher.MatchCallCount()).To(BeZero())
			})
		})

		Context("when the regular matcher returns true", func() {
			BeforeEach(func() {
				matcher.MatchStub = func(string) bool {
					return matcher.MatchCallCount() != 1 // 2 should match
				}
			})

			It("calls the callback with the line", func() {
				var actualLine *scanners.Line
				callback := func(line scanners.Line) error {
					actualLine = &line
					return nil
				}
				sniffer.Sniff(logger, scanner, callback)
				Expect(actualLine).To(Equal(expectedLine))
			})

			Context("when the callback returns an error", func() {
				var (
					callCount int
					callback  func(scanners.Line) error
				)

				BeforeEach(func() {
					callCount = 0

					callback = func(line scanners.Line) error {
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

				scanner.LineStub = func() *scanners.Line {
					scanner.ScanReturns(false)

					return &scanners.Line{
						Content: line,
					}
				}

				sniffer.Sniff(logger, scanner, func(line scanners.Line) error {
					actuals = append(actuals, line.Content)
					return nil
				})
			}

			Expect(actuals).To(ConsistOf(expectations))
		})
	})
})

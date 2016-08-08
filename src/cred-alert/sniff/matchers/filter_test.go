package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/sniff/matchers"
	"cred-alert/sniff/matchers/matchersfakes"
)

var _ = Describe("Filter", func() {
	var (
		filter     matchers.Matcher
		submatcher *matchersfakes.FakeMatcher

		filters []string
	)

	BeforeEach(func() {
		filters = []string{}

		submatcher = &matchersfakes.FakeMatcher{}
	})

	JustBeforeEach(func() {
		filter = matchers.Filter(submatcher, filters...)
	})

	Context("when none of the filters match", func() {
		var line string

		BeforeEach(func() {
			filters = []string{"word", "$"}
			line = "this is a very expensive string to scan"
		})

		It("returns false", func() {
			result := filter.Match([]byte(line))
			Expect(result).To(BeFalse())
		})

		It("does not call the submatcher", func() {
			Expect(submatcher.MatchCallCount()).To(BeZero())
		})
	})

	Context("when at least one of the filters match", func() {
		var line string

		BeforeEach(func() {
			filters = []string{"string", "$"}
			line = "this is a very expensive string to scan"
		})

		It("returns whatever the submatcher returns", func() {
			submatcher.MatchReturns(true)

			result := filter.Match([]byte(line))
			Expect(result).To(BeTrue())
		})
	})
})

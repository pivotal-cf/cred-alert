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
		line    []byte
	)

	BeforeEach(func() {
		filters = []string{}

		submatcher = &matchersfakes.FakeMatcher{}
		line = []byte("this is a very expensive string to scan")
	})

	JustBeforeEach(func() {
		filter = matchers.Filter(submatcher, filters...)
	})

	Context("when none of the filters match", func() {
		BeforeEach(func() {
			filters = []string{"word", "$"}
		})

		It("returns false", func() {
			result, _, _ := filter.Match(line)
			Expect(result).To(BeFalse())
		})

		It("does not call the submatcher", func() {
			Expect(submatcher.MatchCallCount()).To(BeZero())
		})
	})

	Context("when at least one of the filters match", func() {
		BeforeEach(func() {
			filters = []string{"string", "$"}
		})

		It("returns whatever the submatcher returns", func() {
			submatcher.MatchReturns(true, 7, 19)

			result, start, end := filter.Match(line)
			Expect(result).To(BeTrue())
			Expect(start).To(Equal(7))
			Expect(end).To(Equal(19))
		})
	})
})

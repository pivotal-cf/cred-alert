package matchers_test

import (
	"cred-alert/sniff/matchers"
	"cred-alert/sniff/matchers/matchersfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UpcasedMulti", func() {
	var (
		matcher      *matchersfakes.FakeMatcher
		multimatcher matchers.Matcher
	)

	BeforeEach(func() {
		matcher = new(matchersfakes.FakeMatcher)
		multimatcher = matchers.UpcasedMulti(matcher)
	})

	It("calls each matcher with the line", func() {
		multimatcher.Match("this is a line")

		Expect(matcher.MatchCallCount()).To(Equal(1))
		Expect(matcher.MatchArgsForCall(0)).To(Equal("THIS IS A LINE"))
	})

	It("returns false", func() {
		matches := multimatcher.Match("this is a line")

		Expect(matches).To(BeFalse())
	})

	Context("when at least one of the matchers returns true", func() {
		var (
			trueMatcher *matchersfakes.FakeMatcher
		)

		BeforeEach(func() {
			trueMatcher = new(matchersfakes.FakeMatcher)
			trueMatcher.MatchReturns(true)

			multimatcher = matchers.UpcasedMulti(trueMatcher, matcher)
		})

		It("returns true", func() {
			matches := multimatcher.Match("this is a line")

			Expect(matches).To(BeTrue())
		})

		It("doesn't call the later matchers", func() {
			multimatcher.Match("this is a line")

			Expect(matcher.MatchCallCount()).To(BeZero())
		})
	})
})

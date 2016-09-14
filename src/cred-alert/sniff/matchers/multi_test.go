package matchers_test

import (
	"cred-alert/scanners"
	"cred-alert/sniff/matchers"
	"cred-alert/sniff/matchers/matchersfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UpcasedMulti", func() {
	var (
		matcher      *matchersfakes.FakeMatcher
		multimatcher matchers.Matcher

		matches    bool
		start, end int
	)

	BeforeEach(func() {
		matcher = new(matchersfakes.FakeMatcher)
		multimatcher = matchers.UpcasedMulti(matcher)
	})

	JustBeforeEach(func() {
		matches, start, end = multimatcher.Match(&scanners.Line{
			Content:    []byte("this is a line"),
			LineNumber: 42,
			Path:       "the/path",
		})
	})

	It("calls each matcher with the upcased line", func() {
		Expect(matcher.MatchCallCount()).To(Equal(1))

		line := matcher.MatchArgsForCall(0)
		Expect(line.Content).To(Equal([]byte("THIS IS A LINE")))
		Expect(line.LineNumber).To(Equal(42))
		Expect(line.Path).To(Equal("the/path"))
	})

	It("returns false", func() {
		Expect(matches).To(BeFalse())
	})

	Context("when at least one of the matchers returns true", func() {
		BeforeEach(func() {
			trueMatcher := new(matchersfakes.FakeMatcher)
			trueMatcher.MatchReturns(true, 7, 19)

			multimatcher = matchers.UpcasedMulti(trueMatcher, matcher)
		})

		It("returns true", func() {
			Expect(matches).To(BeTrue())
		})

		It("returns offsets", func() {
			Expect(start).To(Equal(7))
			Expect(end).To(Equal(19))
		})

		It("doesn't call the later matchers", func() {
			Expect(matcher.MatchCallCount()).To(BeZero())
		})
	})
})

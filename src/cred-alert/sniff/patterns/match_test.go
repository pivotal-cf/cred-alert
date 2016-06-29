package patterns_test

import (
	"cred-alert/sniff/patterns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pattern matching", func() {
	It("matches a given pattern", func() {
		matcher := patterns.NewMatcher(
			[]string{
				`password`,
				`secret`,
			},
			[]string{
				`fake`,
				`example`,
			},
		)

		found := matcher.Match("blorb")
		Expect(found).To(BeFalse())

		found = matcher.Match("this is my password")
		Expect(found).To(BeTrue())

		found = matcher.Match("too secret for me")
		Expect(found).To(BeTrue())

		found = matcher.Match("fakepassword")
		Expect(found).To(BeFalse())

		found = matcher.Match("examplepassword")
		Expect(found).To(BeFalse())

	})
})

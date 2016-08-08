package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/sniff/matchers"
)

var _ = Describe("Substring", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.Substring("exact match")
	})

	It("returns true when the line matches case-sensitively", func() {
		line := "this is an exact match"
		Expect(matcher.Match([]byte(line))).To(BeTrue())
	})

	It("returns false when the line does not match case-sensitively", func() {
		line := "THIS IS NOT QUITE AN EXACT MATCH"
		Expect(matcher.Match([]byte(line))).To(BeFalse())
	})

	It("returns false when the line does not match", func() {
		line := "this is not exactly a match"
		Expect(matcher.Match([]byte(line))).To(BeFalse())
	})
})

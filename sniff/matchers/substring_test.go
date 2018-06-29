package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/cred-alert/sniff/matchers"
)

var _ = Describe("Substring", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.Substring("exact match")
	})

	It("returns true when the line matches case-sensitively", func() {
		line := []byte("this is an exact match")
		matched, start, end := matcher.Match(line)
		Expect(matched).To(BeTrue())
		Expect(start).To(Equal(11))
		Expect(end).To(Equal(22))
	})

	It("returns false when the line does not match case-sensitively", func() {
		line := []byte("THIS IS NOT QUITE AN EXACT MATCH")
		Expect(matcher.Match(line)).To(BeFalse())
	})

	It("returns false when the line does not match", func() {
		line := []byte("this is not exactly a match")
		Expect(matcher.Match(line)).To(BeFalse())
	})
})

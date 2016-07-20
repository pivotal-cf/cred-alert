package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/sniff/matchers"
)

var _ = Describe("Known Matcher", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.KnownFormat("AKIA[A-Z0-9]{16}")
	})

	It("returns true when the line matches", func() {
		line := "aws_access_key_id: AKIAIOSFOEXAMPLETPWI"
		Expect(matcher.Match(line)).To(BeTrue())
	})

	It("returns false when the line does not match", func() {
		line := "does not match"
		Expect(matcher.Match(line)).To(BeFalse())
	})
})

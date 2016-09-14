package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/scanners"
	"cred-alert/sniff/matchers"
)

var _ = Describe("Format", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.Format("AKIA[A-Z0-9]{16}")
	})

	It("returns true when the line matches case-sensitively", func() {
		line := &scanners.Line{Content: []byte("aws_access_key_id: AKIAIOSFOEXAMPLETPWI")}
		matched, start, end := matcher.Match(line)
		Expect(matched).To(BeTrue())
		Expect(start).To(Equal(19))
		Expect(end).To(Equal(39))
	})

	It("returns false when the line does not match case-sensitively", func() {
		line := &scanners.Line{Content: []byte("aws_access_key_id: akiaiosfoexampletpwi")}
		matched, _, _ := matcher.Match(line)
		Expect(matched).To(BeFalse())
	})

	It("returns false when the line does not match", func() {
		line := &scanners.Line{Content: []byte("does not match")}
		matched, _, _ := matcher.Match(line)
		Expect(matched).To(BeFalse())
	})
})

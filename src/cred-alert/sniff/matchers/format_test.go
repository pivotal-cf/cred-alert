package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/sniff/matchers"
)

var _ = Describe("Format", func() {
	var matcher matchers.Matcher

	var ItBehavesLikeARegexMatcher = func() {
		It("returns true when the line matches case-sensitively", func() {
			line := []byte("aws_access_key_id: AKIAIOSFOEXAMPLETPWI")
			matched, start, end := matcher.Match(line)
			Expect(matched).To(BeTrue())
			Expect(start).To(Equal(19))
			Expect(end).To(Equal(39))
		})

		It("returns false when the line does not match case-sensitively", func() {
			line := []byte("aws_access_key_id: akiaiosfoexampletpwi")
			matched, _, _ := matcher.Match(line)
			Expect(matched).To(BeFalse())
		})

		It("returns false when the line does not match", func() {
			line := []byte("does not match")
			matched, _, _ := matcher.Match(line)
			Expect(matched).To(BeFalse())
		})
	}

	Context("with the standard constructor", func() {
		BeforeEach(func() {
			matcher = matchers.Format("AKIA[A-Z0-9]{16}")
		})

		ItBehavesLikeARegexMatcher()
	})

	Context("with the lenient constructor", func() {
		Context("with a valid regular expression", func() {
			BeforeEach(func() {
				var err error
				matcher, err = matchers.TryFormat("AKIA[A-Z0-9]{16}")
				Expect(err).NotTo(HaveOccurred())
			})

			ItBehavesLikeARegexMatcher()
		})

		Context("with a broken regular expression", func() {
			It("returns an error", func() {
				_, err := matchers.TryFormat("((")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

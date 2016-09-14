package scanners_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/scanners"
)

var _ = Describe("Line/Violation", func() {
	Describe("extracting just the credential out of the line", func() {
		It("let's us extract just the credential", func() {
			violation := scanners.Violation{
				Line: scanners.Line{
					Content: []byte("hello this is a credential"),
				},
				Start: 16,
				End:   26,
			}

			Expect(violation.Credential()).To(Equal("credential"))
		})
	})
})

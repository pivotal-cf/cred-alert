package scanners_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/scanners"
)

var _ = Describe("Violation", func() {
	var violation scanners.Violation

	BeforeEach(func() {
		violation = scanners.Violation{
			Line: scanners.Line{
				Content: []byte("hello this is a credential"),
			},
			Start: 16,
			End:   26,
		}
	})

	Describe("Credential", func() {
		It("returns just the credential portion of the line", func() {
			Expect(violation.Credential()).To(Equal("credential"))
		})
	})
})

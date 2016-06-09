package entropy_test

import (
	"cred-alert/entropy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Entropy Scanning", func() {
	It("finds passwords", func() {
		result := entropy.IsPasswordSuspect("password")
		Expect(result).To(BeFalse())

		result = entropy.IsPasswordSuspect("N9R5tMnaAYKRXgPMWyZsytJt")
		Expect(result).To(BeTrue())
	})
})

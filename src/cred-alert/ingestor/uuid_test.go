package ingestor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/ingestor"
)

var _ = Describe("UUID", func() {
	It("generates different UUIDs each time", func() {
		generator := ingestor.NewGenerator()

		first := generator.Generate()
		second := generator.Generate()

		Expect(first).NotTo(Equal(second))
	})
})

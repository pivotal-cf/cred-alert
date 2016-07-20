package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/queue"
)

var _ = Describe("UUID", func() {
	It("generates different UUIDs each time", func() {
		generator := queue.NewGenerator()

		first := generator.Generate()
		second := generator.Generate()

		Expect(first).NotTo(Equal(second))
	})
})

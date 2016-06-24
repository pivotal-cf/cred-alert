package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestCredAlert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cred-alert Suite")
}

var _ = Describe("cred-alert binary", func() {
	It("compiles", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert")
		Expect(err).NotTo(HaveOccurred())
	})
})

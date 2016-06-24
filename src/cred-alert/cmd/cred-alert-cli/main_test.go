package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestCredAlertCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cred-alert-cli Suite")
}

var _ = Describe("cred-alert-cli binary", func() {
	It("compiles", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert-cli")
		Expect(err).NotTo(HaveOccurred())
	})
})

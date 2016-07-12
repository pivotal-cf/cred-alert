package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestBinaries(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Binaries Suite")
}

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = Describe("Binaries", func() {
	It("builds cred-alert", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds cred-alert-cli", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert-cli")
		Expect(err).NotTo(HaveOccurred())
	})
})

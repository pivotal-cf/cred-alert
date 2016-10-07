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
	It("builds revok-worker", func() {
		_, err := gexec.Build("cred-alert/cmd/revok-worker")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds cred-alert-ingestor", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert-ingestor")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds cred-alert-worker", func() {
		_, err := gexec.Build("cred-alert/cmd/cred-alert-worker")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds stats-monitor", func() {
		_, err := gexec.Build("cred-alert/cmd/stats-monitor")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds revok-ingestor", func() {
		_, err := gexec.Build("cred-alert/cmd/revok-ingestor")
		Expect(err).NotTo(HaveOccurred())
	})
})

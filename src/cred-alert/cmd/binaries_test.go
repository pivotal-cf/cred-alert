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

	It("builds revok-worker-pi", func() {
		_, err := gexec.Build("cred-alert/cmd/revok-worker-api")
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

	It("builds credential-count-publisher", func() {
		_, err := gexec.Build("cred-alert/cmd/credential-count-publisher")
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds srcint", func() {
		_, err := gexec.Build("cred-alert/cmd/srcint")
		Expect(err).NotTo(HaveOccurred())
	})
})

package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestCredAlertCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Suite")
}

var cliPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	cliPath, err = gexec.Build("cred-alert/cmd/cred-alert-cli")
	Expect(err).NotTo(HaveOccurred())

	return []byte(cliPath)
}, func(data []byte) {
	cliPath = string(data)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

package main_test

import (
	"os"
	"strings"
	"time"

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
var oldCliPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	cliPath, err = gexec.Build("github.com/pivotal-cf/cred-alert")
	Expect(err).NotTo(HaveOccurred())

	oldCliPath, err = gexec.Build("github.com/pivotal-cf/cred-alert")
	Expect(err).NotTo(HaveOccurred())

	fifteenDaysAgo := time.Now().Add(-15 * 24 * time.Hour)
	err = os.Chtimes(oldCliPath, fifteenDaysAgo, fifteenDaysAgo)
	Expect(err).NotTo(HaveOccurred())

	return []byte(cliPath + "," + oldCliPath)
}, func(data []byte) {
	parts := strings.Split(string(data), ",")

	cliPath = parts[0]
	oldCliPath = parts[1]
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

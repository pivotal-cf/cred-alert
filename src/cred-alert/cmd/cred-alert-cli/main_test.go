package main_test

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		cmdArgs       []string
		stdin         io.Reader
		session       *gexec.Session
		offendingText = `
			words
			AKIASOMEMORETEXTHERE
			words
		`
	)

	JustBeforeEach(func() {
		cmd := exec.Command(cliPath, cmdArgs...)
		if stdin != nil {
			cmd.Stdin = stdin
		}

		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when given content on stdin", func() {
		BeforeEach(func() {
			stdin = strings.NewReader(offendingText)
		})

		It("scans stdin", func() {
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
		})
	})

	Context("when given a directory flag", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "cli-main-test")
			Expect(err).NotTo(HaveOccurred())

			tmpFile, err := ioutil.TempFile(tmpDir, "cli-main-test")
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			ioutil.WriteFile(tmpFile.Name(), []byte(offendingText), os.ModePerm)

			cmdArgs = []string{"-d", filepath.Dir(tmpFile.Name())}
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("scans each file in the directory", func() {
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
		})
	})

	Context("when given a file flag", func() {
		var tmpFile *os.File

		BeforeEach(func() {
			var err error
			tmpFile, err = ioutil.TempFile("", "cli-main-test")
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			ioutil.WriteFile(tmpFile.Name(), []byte(offendingText), os.ModePerm)

			cmdArgs = []string{"-f", tmpFile.Name()}
		})

		AfterEach(func() {
			os.RemoveAll(tmpFile.Name())
		})

		It("scans the file", func() {
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
		})
	})
})

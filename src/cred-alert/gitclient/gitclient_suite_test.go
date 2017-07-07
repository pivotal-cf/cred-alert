package gitclient_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGitclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gitclient Suite")
}

func git(path string, args ...string) string {
	stdout := &bytes.Buffer{}

	cmd := exec.Command("git", args...)
	cmd.Env = append(
		os.Environ(),
		"TERM=dumb",
		"GIT_COMMITTER_NAME=Korben Dallas",
		"GIT_COMMITTER_EMAIL=korben@git.example.com",
		"GIT_AUTHOR_NAME=Korben Dallas",
		"GIT_AUTHOR_EMAIL=korben@git.example.com",
	)
	cmd.Dir = path
	cmd.Stdout = io.MultiWriter(GinkgoWriter, stdout)
	cmd.Stderr = GinkgoWriter
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	return strings.TrimSpace(stdout.String())
}

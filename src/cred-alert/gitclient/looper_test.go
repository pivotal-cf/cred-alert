package gitclient_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/gitclient"
)

var _ = Describe("Looper", func() {
	var (
		looper gitclient.Looper

		upstreamPath string
		localPath    string
	)

	var git = func(path string, args ...string) string {
		stdout := &bytes.Buffer{}

		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "TERM=dumb")
		cmd.Dir = path
		cmd.Stdout = io.MultiWriter(GinkgoWriter, stdout)
		cmd.Stderr = GinkgoWriter
		err := cmd.Run()
		Expect(err).NotTo(HaveOccurred())

		return strings.TrimSpace(stdout.String())
	}

	var gitUpstream = func(args ...string) string {
		return git(upstreamPath, args...)
	}

	var gitLocal = func(args ...string) string {
		return git(localPath, args...)
	}

	var writeFile = func(path string, contents string) {
		filePath := filepath.Join(upstreamPath, path)
		dir := filepath.Dir(filePath)

		err := os.MkdirAll(dir, 0700)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filePath, []byte(contents), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		looper = gitclient.NewLooper()

		var err error
		upstreamPath, err = ioutil.TempDir("", "repo-upstream")
		Expect(err).NotTo(HaveOccurred())

		localPath, err = ioutil.TempDir("", "repo-local")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(upstreamPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("scans multiple files", func() {
		gitUpstream("init")

		writeFile(filepath.Join("dir", "my-special-file.txt"), "My Special Data")
		writeFile("my-boring-file.txt", "boring data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "Initial commit")
		expectedSha := gitUpstream("rev-list", "HEAD", "--max-count=1")

		gitLocal("clone", upstreamPath, ".")

		shas := []string{}
		paths := []string{}
		contents := [][]byte{}

		err := looper.ScanCurrentState(localPath, func(sha string, path string, content []byte) {
			shas = append(shas, sha)
			paths = append(paths, path)
			contents = append(contents, content)
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(shas).To(ConsistOf(expectedSha, expectedSha))
		Expect(paths).To(ConsistOf("dir/my-special-file.txt", "my-boring-file.txt"))
		Expect(contents).To(ConsistOf([]byte("My Special Data"), []byte("boring data")))
	})

	It("only scans the most recent commit on a branch", func() {
		gitUpstream("init")

		writeFile("my-special-file.txt", "first data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "Initial commit")

		writeFile("my-special-file.txt", "second data")

		gitUpstream("commit", "-am", "Second commit")

		expectedSha := gitUpstream("rev-list", "HEAD", "--max-count=1")

		gitLocal("clone", upstreamPath, ".")

		callbackCallCount := 0

		err := looper.ScanCurrentState(localPath, func(sha string, path string, content []byte) {
			callbackCallCount++

			Expect(sha).To(Equal(expectedSha))
			Expect(path).To(Equal("my-special-file.txt"))
			Expect(content).To(Equal([]byte("second data")))
		})

		Expect(err).NotTo(HaveOccurred())

		Expect(callbackCallCount).To(Equal(1))
	})

	It("scans multiple branches", func() {
		gitUpstream("init")

		writeFile("my-special-file.txt", "first data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "Initial commit")

		expectedSha1 := gitUpstream("rev-list", "HEAD", "--max-count=1")

		gitUpstream("checkout", "-b", "my-special-branch")

		writeFile("my-special-file.txt", "second data")
		writeFile("other-file.txt", "other data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "Second commit")

		expectedSha2 := gitUpstream("rev-list", "HEAD", "--max-count=1")

		gitLocal("clone", upstreamPath, ".")
		gitLocal("fetch")

		shas := []string{}
		paths := []string{}
		contents := [][]byte{}

		err := looper.ScanCurrentState(localPath, func(sha string, path string, content []byte) {
			shas = append(shas, sha)
			paths = append(paths, path)
			contents = append(contents, content)
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(shas).To(ConsistOf(expectedSha1, expectedSha2, expectedSha2))
		Expect(paths).To(ConsistOf("my-special-file.txt", "my-special-file.txt", "other-file.txt"))
		Expect(contents).To(ConsistOf([]byte("first data"), []byte("second data"), []byte("other data")))
	})
})

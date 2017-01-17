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

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
)

var _ = Describe("Looper", func() {
	var (
		looper gitclient.Looper

		repoRepository *dbfakes.FakeRepositoryRepository
		repoPath       string
	)

	var git = func(args ...string) string {
		stdout := &bytes.Buffer{}

		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "TERM=dumb")
		cmd.Dir = repoPath
		cmd.Stdout = io.MultiWriter(GinkgoWriter, stdout)
		cmd.Stderr = GinkgoWriter
		err := cmd.Run()
		Expect(err).NotTo(HaveOccurred())

		return strings.TrimSpace(stdout.String())
	}

	var writeFile = func(path string, contents string) {
		filePath := filepath.Join(repoPath, path)
		dir := filepath.Dir(filePath)

		err := os.MkdirAll(dir, 0700)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filePath, []byte(contents), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		repoRepository = &dbfakes.FakeRepositoryRepository{}

		looper = gitclient.NewLooper(repoRepository)

		var err error
		repoPath, err = ioutil.TempDir("", "repo")
		Expect(err).NotTo(HaveOccurred())

		repoRepository.FindReturns(db.Repository{
			Path: repoPath,
		}, nil)
	})

	AfterEach(func() {
		err := os.RemoveAll(repoPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("scans multiple files", func() {
		git("init")

		writeFile(filepath.Join("dir", "my-special-file.txt"), "My Special Data")
		writeFile("my-boring-file.txt", "boring data")

		git("add", ".")
		git("commit", "-m", "Initial commit")
		expectedSha := git("rev-list", "HEAD", "--max-count=1")

		shas := []string{}
		paths := []string{}
		contents := [][]byte{}

		err := looper.ScanCurrentState("some-owner", "some-repo", func(sha string, path string, content []byte) {
			shas = append(shas, sha)
			paths = append(paths, path)
			contents = append(contents, content)
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(shas).To(ConsistOf(expectedSha, expectedSha))
		Expect(paths).To(ConsistOf("dir/my-special-file.txt", "my-boring-file.txt"))
		Expect(contents).To(ConsistOf([]byte("My Special Data"), []byte("boring data")))

		Expect(repoRepository.FindCallCount()).To(Equal(1))
		passedOwner, passedRepo := repoRepository.FindArgsForCall(0)
		Expect(passedOwner).To(Equal("some-owner"))
		Expect(passedRepo).To(Equal("some-repo"))
	})

	It("only scans the most recent commit on a branch", func() {
		git("init")

		writeFile("my-special-file.txt", "first data")

		git("add", ".")
		git("commit", "-m", "Initial commit")

		writeFile("my-special-file.txt", "second data")

		git("commit", "-am", "Second commit")

		expectedSha := git("rev-list", "HEAD", "--max-count=1")

		callbackCallCount := 0

		err := looper.ScanCurrentState("some-owner", "some-repo", func(sha string, path string, content []byte) {
			callbackCallCount++

			Expect(sha).To(Equal(expectedSha))
			Expect(path).To(Equal("my-special-file.txt"))
			Expect(content).To(Equal([]byte("second data")))
		})

		Expect(err).NotTo(HaveOccurred())

		Expect(callbackCallCount).To(Equal(1))
	})

	It("scans multiple branches", func() {
		git("init")

		writeFile("my-special-file.txt", "first data")

		git("add", ".")
		git("commit", "-m", "Initial commit")

		expectedSha1 := git("rev-list", "HEAD", "--max-count=1")

		git("checkout", "-b", "my-special-branch")

		writeFile("my-special-file.txt", "second data")
		writeFile("other-file.txt", "other data")

		git("add", ".")
		git("commit", "-m", "Second commit")

		expectedSha2 := git("rev-list", "HEAD", "--max-count=1")

		shas := []string{}
		paths := []string{}
		contents := [][]byte{}

		err := looper.ScanCurrentState("some-owner", "some-repo", func(sha string, path string, content []byte) {
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

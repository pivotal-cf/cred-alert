package gitclient_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/gitclient"
)

var _ = Describe("Looper", func() {
	var (
		fileLookup gitclient.FileLookup

		upstreamPath string
		localPath    string
	)

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
		fileLookup = gitclient.NewFileLookup()

		var err error
		upstreamPath, err = ioutil.TempDir("", "repo-upstream")
		Expect(err).NotTo(HaveOccurred())

		localPath, err = ioutil.TempDir("", "repo-local")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(upstreamPath)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(localPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("scans multiple files", func() {
		gitUpstream("init")

		writeFile(filepath.Join("config", "blobs.yml"), "blobs data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "Initial commit")

		gitUpstream("checkout", "-b", "other-branch")

		writeFile(filepath.Join("config", "blobs.yml"), "new blobs data")

		gitUpstream("add", ".")
		gitUpstream("commit", "-m", "new commit")

		gitLocal("clone", upstreamPath, ".")

		contentBytes, err := fileLookup.FileContents(localPath, "other-branch", "config/blobs.yml")
		Expect(err).NotTo(HaveOccurred())
		Expect(contentBytes).To(Equal([]byte("new blobs data")))
	})
})

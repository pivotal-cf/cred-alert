package rolodex_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/gitclient"
	"rolodex"
	"rolodex/rolodexfakes"
)

var _ = Describe("Syncer", func() {
	var (
		teamRepository *rolodexfakes.FakeTeamRepository
		logger         *lagertest.TestLogger

		syncer rolodex.Syncer

		workDir      string
		upstreamPath string
		localPath    string
	)

	var git = func(path string, args ...string) string {
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

	var gitUpstream = func(args ...string) string {
		return git(upstreamPath, args...)
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
		logger = lagertest.NewTestLogger("syncer")

		gitClient := gitclient.New("", "")

		teamRepository = &rolodexfakes.FakeTeamRepository{}

		var err error

		workDir, err = ioutil.TempDir("", "syncer_workdir")
		Expect(err).NotTo(HaveOccurred())

		upstreamPath = filepath.Join(workDir, "origin")
		err = os.MkdirAll(upstreamPath, 0700)
		Expect(err).NotTo(HaveOccurred())

		localPath = filepath.Join(workDir, "local")

		syncer = rolodex.NewSyncer(logger, upstreamPath, localPath, gitClient, teamRepository)
	})

	AfterEach(func() {
		err := os.RemoveAll(workDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Sync", func() {
		Context("when there is a master branch", func() {
			BeforeEach(func() {
				writeFile("README.md", "I SAID READ ME!!!")

				gitUpstream("init")
				gitUpstream("add", "README.md")
				gitUpstream("commit", "-m", "Initial commit")
			})

			Context("when the repository has never been cloned", func() {
				BeforeEach(func() {
					syncer.Sync()
				})

				It("clones the repository", func() {
					path := filepath.Join(localPath, "README.md")
					Expect(path).To(BeAnExistingFile())
				})

				It("tells the team repository to update", func() {
					Expect(teamRepository.ReloadCallCount()).To(Equal(1)) // clone
				})
			})

			Context("when the repository has already been cloned", func() {
				Context("when there are changes", func() {
					BeforeEach(func() {
						syncer.Sync()

						writeFile("LICENSE", "You can't have it")

						gitUpstream("add", "LICENSE")
						gitUpstream("commit", "-m", "Second commit")

						syncer.Sync()
					})

					It("updates the repository", func() {
						path := filepath.Join(localPath, "LICENSE")
						Expect(path).To(BeAnExistingFile())
					})

					It("tells the team repository to update", func() {
						Expect(teamRepository.ReloadCallCount()).To(Equal(2)) // clone and fetch
					})
				})

				Context("when there are no changes", func() {
					BeforeEach(func() {
						syncer.Sync()
						syncer.Sync()
					})

					It("does not tell the team repository to update", func() {
						Expect(teamRepository.ReloadCallCount()).To(Equal(1)) // clone
					})
				})
			})
		})

		Context("when there is no master branch", func() {
			BeforeEach(func() {
				writeFile("README.md", "I SAID READ ME!!!")

				gitUpstream("init")
				gitUpstream("checkout", "-b", "my-special-branch")
				gitUpstream("add", "README.md")
				gitUpstream("commit", "-m", "Initial commit")

				syncer.Sync()

				writeFile("LICENSE", "You can't have it")

				gitUpstream("add", "LICENSE")
				gitUpstream("commit", "-m", "Second commit")

				syncer.Sync()
			})

			It("should log", func() {
				Expect(logger).To(gbytes.Say("no remote master branch found"))
			})

			It("does not tell the team repository to update", func() {
				Expect(teamRepository.ReloadCallCount()).To(Equal(1)) // clone
			})
		})
	})
})

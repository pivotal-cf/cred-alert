package rolodex_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"rolodex"
	"rolodex/rolodexfakes"
)

var _ = Describe("Syncer", func() {
	var (
		teamRepository *rolodexfakes.FakeTeamRepository
		logger         *lagertest.TestLogger
		emitter        *metricsfakes.FakeEmitter

		syncer rolodex.Syncer

		workDir      string
		upstreamPath string
		localPath    string

		successCounter *metricsfakes.FakeCounter
		failureCounter *metricsfakes.FakeCounter
		fetchTimer     *metricsfakes.FakeTimer
	)

	var runGit = func(path string, args ...string) string {
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
		return runGit(upstreamPath, args...)
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
		emitter = &metricsfakes.FakeEmitter{}
		successCounter = &metricsfakes.FakeCounter{}
		failureCounter = &metricsfakes.FakeCounter{}
		fetchTimer = &metricsfakes.FakeTimer{}

		emitter.CounterStub = func(name string) metrics.Counter {
			if name == "rolodex.syncer.fetch.success" {
				return successCounter
			}

			if name == "rolodex.syncer.fetch.failure" {
				return failureCounter
			}

			panic("did not expect counter called: " + name)
		}

		fetchTimer.TimeStub = func(logger lager.Logger, fn func(), tags ...string) {
			fn()
		}

		emitter.TimerReturns(fetchTimer)

		gitClient := gitclient.New("", "", "")

		teamRepository = &rolodexfakes.FakeTeamRepository{}

		var err error

		workDir, err = ioutil.TempDir("", "syncer_workdir")
		Expect(err).NotTo(HaveOccurred())

		upstreamPath = filepath.Join(workDir, "origin")
		err = os.MkdirAll(upstreamPath, 0700)
		Expect(err).NotTo(HaveOccurred())

		localPath = filepath.Join(workDir, "local")

		syncer = rolodex.NewSyncer(logger, emitter, upstreamPath, localPath, gitClient, teamRepository)
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
				BeforeEach(func() {
					syncer.Sync()
				})

				Context("when there are changes", func() {
					BeforeEach(func() {
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

					It("increments the success counter", func() {
						Expect(successCounter.IncCallCount()).To(Equal(1))
					})

					It("times the fetch", func() {
						Expect(fetchTimer.TimeCallCount()).To(Equal(1))
					})
				})

				Context("when there are no changes", func() {
					BeforeEach(func() {
						syncer.Sync()
					})

					It("increments the success counter", func() {
						Expect(successCounter.IncCallCount()).To(Equal(1))
					})
				})

				Context("when the subsequent fetch fails", func() {
					BeforeEach(func() {
						err := os.RemoveAll(upstreamPath)
						Expect(err).NotTo(HaveOccurred())

						syncer.Sync()
					})

					It("increments the failure counter", func() {
						Expect(failureCounter.IncCallCount()).To(Equal(1))
					})
				})

				Context("when resetting the state fails", func() {
					var (
						gitClient *rolodexfakes.FakeGitSyncClient
					)

					BeforeEach(func() {
						gitClient = &rolodexfakes.FakeGitSyncClient{}
						syncer = rolodex.NewSyncer(logger, emitter, upstreamPath, localPath, gitClient, teamRepository)

						sha := "4d70bfc4198320f1aa04cd474eb71af2d24cfa48"

						gitClient.FetchReturns(map[string][]string{
							"refs/remotes/origin/master": {sha, sha},
						}, nil)

						gitClient.HardResetReturns(errors.New("disaster"))

						syncer.Sync()
					})

					It("increments the failure counter", func() {
						Expect(failureCounter.IncCallCount()).To(Equal(1))
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

			It("increments the failure counter", func() {
				Expect(failureCounter.IncCallCount()).To(Equal(1))
			})
		})
	})
})

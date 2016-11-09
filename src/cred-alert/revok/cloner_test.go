package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/lagertest"
	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cloner", func() {
	var (
		workdir              string
		workCh               chan revok.CloneMsg
		logger               *lagertest.TestLogger
		gitClient            gitclient.Client
		repositoryRepository *dbfakes.FakeRepositoryRepository
		emitter              *metricsfakes.FakeEmitter
		scanner              *revokfakes.FakeScanner

		scanSuccessMetric  *metricsfakes.FakeCounter
		scanFailedMetric   *metricsfakes.FakeCounter
		cloneSuccessMetric *metricsfakes.FakeCounter
		cloneFailedMetric  *metricsfakes.FakeCounter
		repoPath           string
		repo               *git.Repository
		potatoesHeadSHA    string
		tomatoesHeadSHA    string

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		workCh = make(chan revok.CloneMsg, 10)
		gitClient = gitclient.New("private-key-path", "public-key-path")
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		repositoryRepository.FindReturns(db.Repository{
			Model: db.Model{
				ID: 42,
			},
		}, nil)

		emitter = &metricsfakes.FakeEmitter{}
		scanSuccessMetric = &metricsfakes.FakeCounter{}
		scanFailedMetric = &metricsfakes.FakeCounter{}
		cloneSuccessMetric = &metricsfakes.FakeCounter{}
		cloneFailedMetric = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.cloner.scan.success":
				return scanSuccessMetric
			case "revok.cloner.scan.failed":
				return scanFailedMetric
			case "revok.cloner.clone.success":
				return cloneSuccessMetric
			case "revok.cloner.clone.failed":
				return cloneFailedMetric
			}
			return &metricsfakes.FakeCounter{}
		}

		scanner = &revokfakes.FakeScanner{}

		var err error
		repoPath, err = ioutil.TempDir("", "revok-test-base-repo")
		Expect(err).NotTo(HaveOccurred())

		repo, err = git.InitRepository(repoPath, false)
		Expect(err).NotTo(HaveOccurred())

		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		repo.Free()
		ginkgomon.Interrupt(process)
		<-process.Wait()

		os.RemoveAll(repoPath)
		os.RemoveAll(workdir)
	})

	JustBeforeEach(func() {
		runner = revok.NewCloner(
			logger,
			workdir,
			workCh,
			gitClient,
			repositoryRepository,
			scanner,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	It("does not try to scan when there are no branches", func() {
		Consistently(scanner.ScanCallCount).Should(BeZero())
	})

	Context("when there are multiple branches", func() {
		BeforeEach(func() {
			result := createCommit("refs/heads/potatoes", repoPath, "some-potato", []byte("credential"), "Initial commit on potatoes", nil)
			potatoesHeadSHA = result.To.String()
			result = createCommit("refs/heads/tomatoes", repoPath, "some-tomato", []byte("credential"), "Initial commit on tomatoes", nil)
			tomatoesHeadSHA = result.To.String()
		})

		Context("when there is a message on the clone message channel", func() {
			BeforeEach(func() {
				workCh <- revok.CloneMsg{
					URL:        repoPath,
					Repository: "some-repo",
					Owner:      "some-owner",
				}
			})

			It("clones the repository to workdir/owner/repo", func() {
				Eventually(filepath.Join(workdir, "some-owner", "some-repo", ".git")).Should(BeADirectory())
				Eventually(func() error {
					_, err := git.OpenRepository(filepath.Join(workdir, "some-owner", "some-repo"))
					return err
				}).ShouldNot(HaveOccurred())
			})

			It("finds the repository in the database that matches the owner and repo", func() {
				Eventually(repositoryRepository.FindCallCount).Should(Equal(1))
				owner, name := repositoryRepository.FindArgsForCall(0)
				Expect(owner).To(Equal("some-owner"))
				Expect(name).To(Equal("some-repo"))
			})

			It("marks the repository in the database as cloned", func() {
				Eventually(repositoryRepository.MarkAsClonedCallCount).Should(Equal(1))
			})

			It("tries to scan all branches", func() {
				Eventually(scanner.ScanCallCount).Should(Equal(2))

				var startSHAs []string
				var branches []string
				var actualScannedOids []map[git.Oid]struct{}
				for i := 0; i < scanner.ScanCallCount(); i++ {
					_, owner, repository, scannedOids, branch, startSHA, stopSHA := scanner.ScanArgsForCall(i)
					Expect(owner).To(Equal("some-owner"))
					Expect(repository).To(Equal("some-repo"))
					Expect(stopSHA).To(Equal(""))

					startSHAs = append(startSHAs, startSHA)
					actualScannedOids = append(actualScannedOids, scannedOids)
					branches = append(branches, branch)
				}

				Expect(startSHAs).To(ConsistOf(tomatoesHeadSHA, potatoesHeadSHA))
				Expect(fmt.Sprintf("%p", actualScannedOids[0])).To(Equal(fmt.Sprintf("%p", actualScannedOids[1])))
				Expect(branches).To(ConsistOf("origin/tomatoes", "origin/potatoes"))
			})

			It("increments the successful clone metric", func() {
				Eventually(cloneSuccessMetric.IncCallCount).Should(Equal(1))
			})

			It("increments the successful scan metric", func() {
				Eventually(scanSuccessMetric.IncCallCount).Should(Equal(2))
			})

			Context("when scanning fails", func() {
				BeforeEach(func() {
					scanner.ScanReturns(errors.New("an-error"))
				})

				It("increments the failed scan metric", func() {
					Eventually(scanFailedMetric.IncCallCount).Should(Equal(2))
				})
			})

			Context("when cloning fails", func() {
				BeforeEach(func() {
					fakeGitClient := &gitclientfakes.FakeClient{}
					fakeGitClient.CloneStub = func(url, dest string) (*git.Repository, error) {
						err := os.MkdirAll(dest, os.ModePerm)
						Expect(err).NotTo(HaveOccurred())
						return nil, errors.New("an-error")
					}
					gitClient = fakeGitClient
				})

				It("cleans up the failed clone destination, if any", func() {
					fakeGitClient, ok := gitClient.(*gitclientfakes.FakeClient)
					Expect(ok).To(BeTrue())

					Eventually(fakeGitClient.CloneCallCount).Should(Equal(1))
					_, dest := fakeGitClient.CloneArgsForCall(0)
					Eventually(dest).ShouldNot(BeADirectory())
				})

				It("does not mark the repository as having been cloned", func() {
					Consistently(repositoryRepository.MarkAsClonedCallCount).Should(BeZero())
				})

				It("does not try to scan", func() {
					Consistently(scanner.ScanCallCount).Should(BeZero())
				})

				It("increments the failed clone metric", func() {
					Eventually(cloneFailedMetric.IncCallCount).Should(Equal(1))
				})
			})

			Context("when marking the repository as cloned fails", func() {
				BeforeEach(func() {
					repositoryRepository.MarkAsClonedReturns(errors.New("an-error"))
				})

				It("does not try to scan", func() {
					Consistently(scanner.ScanCallCount).Should(BeZero())
				})
			})

			Context("when finding the repository fails", func() {
				BeforeEach(func() {
					repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
				})

				It("does not try to scan", func() {
					Consistently(scanner.ScanCallCount).Should(BeZero())
				})
			})
		})
	})
})

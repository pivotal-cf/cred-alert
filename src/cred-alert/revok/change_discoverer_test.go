package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChangeDiscoverer", func() {
	var (
		logger               *lagertest.TestLogger
		gitClient            gitclient.Client
		clock                *fakeclock.FakeClock
		interval             time.Duration
		sniffer              *snifffakes.FakeSniffer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		fetchRepository      *dbfakes.FakeFetchRepository
		scanRepository       *dbfakes.FakeScanRepository
		emitter              *metricsfakes.FakeEmitter

		firstScan      *dbfakes.FakeActiveScan
		secondScan     *dbfakes.FakeActiveScan
		currentFetchID uint

		fetchTimer        *metricsfakes.FakeTimer
		reposToFetch      *metricsfakes.FakeGauge
		runCounter        *metricsfakes.FakeCounter
		successCounter    *metricsfakes.FakeCounter
		failedCounter     *metricsfakes.FakeCounter
		failedScanCounter *metricsfakes.FakeCounter
		failedDiffCounter *metricsfakes.FakeCounter

		remoteRepoPath  string
		repoToFetchPath string
		repoToFetch     *git.Repository

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		gitClient = gitclient.New("private-key-path", "public-key-path")
		clock = fakeclock.NewFakeClock(time.Now())
		interval = 30 * time.Minute

		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			for s.Scan(logger) {
				line := s.Line(logger)
				if strings.Contains(string(line.Content), "credential") {
					h(l, scanners.Violation{
						Line: *line,
					})
				}
			}

			return nil
		}

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		currentFetchID = 0
		fetchRepository = &dbfakes.FakeFetchRepository{}
		fetchRepository.SaveFetchStub = func(l lager.Logger, f *db.Fetch) error {
			currentFetchID++
			f.ID = currentFetchID
			return nil
		}

		scanRepository = &dbfakes.FakeScanRepository{}
		firstScan = &dbfakes.FakeActiveScan{}
		secondScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartStub = func(lager.Logger, string, *db.Repository, *db.Fetch) db.ActiveScan {
			if scanRepository.StartCallCount() == 1 {
				return firstScan
			}
			return secondScan
		}

		emitter = &metricsfakes.FakeEmitter{}
		runCounter = &metricsfakes.FakeCounter{}
		successCounter = &metricsfakes.FakeCounter{}
		failedCounter = &metricsfakes.FakeCounter{}
		failedScanCounter = &metricsfakes.FakeCounter{}
		failedDiffCounter = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.change_discoverer_runs":
				return runCounter
			case "revok.change_discoverer_success":
				return successCounter
			case "revok.change_discoverer_failed":
				return failedCounter
			case "revok.change_discoverer_failed_scans":
				return failedScanCounter
			case "revok.change_discoverer_failed_diffs":
				return failedDiffCounter
			default:
				return &metricsfakes.FakeCounter{}
			}
		}

		fetchTimer = &metricsfakes.FakeTimer{}
		fetchTimer.TimeStub = func(logger lager.Logger, f func(), tags ...string) {
			f()
		}
		emitter.TimerReturns(fetchTimer)
		reposToFetch = &metricsfakes.FakeGauge{}
		emitter.GaugeReturns(reposToFetch)

		var err error
		remoteRepoPath, err = ioutil.TempDir("", "change-discoverer-remote-repo")
		Expect(err).NotTo(HaveOccurred())

		remoteRepo, err := git.InitRepository(remoteRepoPath, false)
		Expect(err).NotTo(HaveOccurred())
		defer remoteRepo.Free()

		createCommit(remoteRepoPath, "some-file", []byte("credential"), "Initial commit")

		repoToFetchPath, err = ioutil.TempDir("", "change-discoverer-repo-to-fetch")
		Expect(err).NotTo(HaveOccurred())

		repoToFetch, err = git.Clone(remoteRepoPath, repoToFetchPath, &git.CloneOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer repoToFetch.Free()
	})

	JustBeforeEach(func() {
		runner = revok.NewChangeDiscoverer(
			logger,
			gitClient,
			clock,
			interval,
			sniffer,
			repositoryRepository,
			fetchRepository,
			scanRepository,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
	})

	It("increments the run metric", func() {
		Eventually(runCounter.IncCallCount).Should(Equal(1))
	})

	It("tries to get repositories from the database immediately on start", func() {
		Eventually(repositoryRepository.NotFetchedSinceCallCount).Should(Equal(1))
		actualTime := repositoryRepository.NotFetchedSinceArgsForCall(0)
		Expect(actualTime).To(Equal(clock.Now().Add(-30 * time.Minute)))
	})

	It("tries to get repositories from the database on a timer", func() {
		Eventually(repositoryRepository.NotFetchedSinceCallCount).Should(Equal(1))
		Consistently(repositoryRepository.NotFetchedSinceCallCount).Should(Equal(1))

		clock.Increment(interval)

		Eventually(repositoryRepository.NotFetchedSinceCallCount).Should(Equal(2))
		Consistently(repositoryRepository.NotFetchedSinceCallCount).Should(Equal(2))
	})

	Context("when there are repositories to fetch", func() {
		BeforeEach(func() {
			repositoryRepository.NotFetchedSinceReturns([]db.Repository{
				{
					Model: db.Model{
						ID: 42,
					},
					Owner: "some-owner",
					Name:  "some-repo",
					Path:  repoToFetchPath,
				},
			}, nil)
		})

		It("increments the repositories to fetch metric", func() {
			Eventually(reposToFetch.UpdateCallCount).Should(Equal(1))
		})

		It("increments the success metric", func() {
			Eventually(successCounter.IncCallCount).Should(Equal(1))
		})

		Context("when the remote has changes", func() {
			BeforeEach(func() {
				createCommit(remoteRepoPath, "some-other-file", []byte("credential"), "second commit")
			})

			It("scans the changes", func() {
				// this is the only way to know we've scanned
				Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
			})

			It("tries to store information in the database about the fetch", func() {
				Eventually(fetchRepository.SaveFetchCallCount).Should(Equal(1))
				_, fetch := fetchRepository.SaveFetchArgsForCall(0)
				Expect(fetch.Path).To(Equal(repoToFetchPath))
				Expect(fetch.Repository.ID).To(BeNumerically(">", 0))

				repo, err := git.OpenRepository(remoteRepoPath)
				Expect(err).NotTo(HaveOccurred())
				defer repo.Free()

				head, err := repo.Head()
				Expect(err).NotTo(HaveOccurred())
				defer head.Free()

				targetRef, err := repo.Lookup(head.Target())
				Expect(err).NotTo(HaveOccurred())
				defer targetRef.Free()

				headCommit, err := targetRef.AsCommit()
				Expect(err).NotTo(HaveOccurred())
				defer headCommit.Free()

				expectedChanges := map[string][]*git.Oid{
					"refs/remotes/origin/master": []*git.Oid{headCommit.ParentId(0), head.Target()},
				}

				bs, err := json.Marshal(expectedChanges)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetch.Changes).To(Equal(bs))
			})

			It("tries to store information in the database about found credentials", func() {
				Eventually(scanRepository.StartCallCount).Should(Equal(1))
				_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
				Expect(scanType).To(Equal("diff-scan"))
				Expect(repository.ID).To(BeNumerically("==", 42))
				Expect(fetch.ID).To(Equal(currentFetchID))

				Eventually(firstScan.FinishCallCount).Should(Equal(1))
			})

			Context("when there is an error saving the fetch to the database", func() {
				BeforeEach(func() {
					fakeGitClient := &gitclientfakes.FakeClient{}
					fetchRepository.SaveFetchReturns(errors.New("an-error"))
					gitClient = fakeGitClient
				})

				It("does not try to diff anything", func() {
					Expect(fetchRepository.SaveFetchCallCount()).To(Equal(1))
					fakeGitClient, ok := gitClient.(*gitclientfakes.FakeClient)
					Expect(ok).To(BeTrue())
					Consistently(fakeGitClient.DiffCallCount).Should(BeZero())
				})

				It("increments the failed metric", func() {
					Eventually(failedCounter.IncCallCount).Should(Equal(1))
				})
			})

			Context("when there is an error storing credentials in the database", func() {
				BeforeEach(func() {
					firstScan.FinishReturns(errors.New("an-error"))
				})

				It("increments the failed scan metric", func() {
					Eventually(firstScan.FinishCallCount).Should(Equal(1))
					Expect(failedScanCounter.IncCallCount()).To(Equal(1))
				})
			})

			Context("when there is an error getting a diff from Git", func() {
				BeforeEach(func() {
					fakeGitClient := &gitclientfakes.FakeClient{}

					// fake client requires successful Fetch
					oldOid, err := git.NewOid("fce98866a7d559757a0a501aa548e244a46ad00a")
					Expect(err).NotTo(HaveOccurred())
					newOid, err := git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
					Expect(err).NotTo(HaveOccurred())
					fakeGitClient.FetchReturns(map[string][]*git.Oid{
						"refs/remotes/origin/master": {oldOid, newOid},
					}, nil)

					fakeGitClient.DiffReturns("", errors.New("an-error"))
					gitClient = fakeGitClient
				})

				It("increments the failed diff metric", func() {
					Eventually(failedDiffCounter.IncCallCount).Should(Equal(1))
				})
			})
		})

		Context("when there is an error fetching changes", func() {
			BeforeEach(func() {
				fakeGitClient := &gitclientfakes.FakeClient{}
				fakeGitClient.FetchReturns(nil, errors.New("an-error"))
				gitClient = fakeGitClient
			})

			It("increments the failed metric", func() {
				Eventually(failedCounter.IncCallCount).Should(Equal(1))
			})
		})
	})

	Context("when there are multiple repositories to fetch", func() {
		var (
			repositories  []db.Repository
			fakeGitClient *gitclientfakes.FakeClient
		)

		BeforeEach(func() {
			repositories = []db.Repository{
				{
					Model: db.Model{
						ID: 42,
					},
					Owner: "some-owner",
					Name:  "some-repo",
					Path:  "some-path",
				},
				{
					Model: db.Model{
						ID: 44,
					},
					Owner: "some-other-owner",
					Name:  "some-other-repo",
					Path:  "some-other-path",
				},
			}

			repositoryRepository.NotFetchedSinceStub = func(time.Time) ([]db.Repository, error) {
				if repositoryRepository.NotFetchedSinceCallCount() == 1 {
					return repositories, nil
				}

				return []db.Repository{}, nil
			}

			fakeGitClient = &gitclientfakes.FakeClient{}
			gitClient = fakeGitClient
		})

		It("waits between fetches", func() {
			Eventually(fakeGitClient.FetchCallCount).Should(Equal(1))
			Consistently(fakeGitClient.FetchCallCount).Should(Equal(1))

			subInterval := time.Duration(interval.Nanoseconds()/int64(len(repositories))) * time.Nanosecond
			clock.Increment(subInterval)

			Eventually(fakeGitClient.FetchCallCount).Should(Equal(2))
		})
	})

	Context("when there is an error getting repositories to fetch", func() {
		var (
			remoteRefPath string
			oldTarget     string
		)

		BeforeEach(func() {
			remoteRefPath = filepath.Join(repoToFetchPath, ".git", "refs", "remotes", "origin", "master")
			bs, err := ioutil.ReadFile(remoteRefPath)
			Expect(err).NotTo(HaveOccurred())
			oldTarget = string(bs)

			createCommit(remoteRepoPath, "some-other-file", []byte("credential"), "second commit")

			repositoryRepository.NotFetchedSinceReturns(nil, errors.New("an-error"))
		})

		It("does not increment the repositories to fetch metric", func() {
			Consistently(reposToFetch.UpdateCallCount).Should(BeZero())
		})

		It("does not do any fetches", func() {
			Consistently(func() string {
				bs, err := ioutil.ReadFile(remoteRefPath)
				Expect(err).NotTo(HaveOccurred())
				return string(bs)
			}).Should(Equal(oldTarget))
		})

		It("does not save any fetches", func() {
			Consistently(fetchRepository.SaveFetchCallCount).Should(BeZero())
		})
	})
})

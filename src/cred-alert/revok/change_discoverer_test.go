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
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("ChangeDiscoverer", func() {
	var (
		logger               *lagertest.TestLogger
		gitClient            gitclient.Client
		clock                *fakeclock.FakeClock
		interval             time.Duration
		scanner              *revokfakes.FakeScanner
		repositoryRepository *dbfakes.FakeRepositoryRepository
		fetchRepository      *dbfakes.FakeFetchRepository
		fetchIntervalUpdater *revokfakes.FakeFetchIntervalUpdater
		emitter              *metricsfakes.FakeEmitter

		currentFetchID       uint
		fetchedRepositoryIds []uint

		fetchTimer          *metricsfakes.FakeTimer
		reposToFetch        *metricsfakes.FakeGauge
		runCounter          *metricsfakes.FakeCounter
		fetchSuccessCounter *metricsfakes.FakeCounter
		fetchFailedCounter  *metricsfakes.FakeCounter
		scanSuccessCounter  *metricsfakes.FakeCounter
		scanFailedCounter   *metricsfakes.FakeCounter

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

		scanner = &revokfakes.FakeScanner{}

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		fetchedRepositoryIds = []uint{}
		currentFetchID = 0

		fetchRepository = &dbfakes.FakeFetchRepository{}
		fetchRepository.RegisterFetchStub = func(l lager.Logger, f *db.Fetch) error {
			currentFetchID++
			f.ID = currentFetchID

			fetchedRepositoryIds = append(fetchedRepositoryIds, f.Repository.ID)

			return nil
		}

		fetchIntervalUpdater = &revokfakes.FakeFetchIntervalUpdater{}

		fetchIntervalUpdater.UpdateFetchIntervalStub = func(repo *db.Repository) error {
			defer GinkgoRecover()
			Expect(fetchedRepositoryIds).To(ContainElement(repo.ID))
			return nil
		}

		emitter = &metricsfakes.FakeEmitter{}
		runCounter = &metricsfakes.FakeCounter{}
		fetchSuccessCounter = &metricsfakes.FakeCounter{}
		fetchFailedCounter = &metricsfakes.FakeCounter{}
		scanSuccessCounter = &metricsfakes.FakeCounter{}
		scanFailedCounter = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.change_discoverer.run":
				return runCounter
			case "revok.change_discoverer.fetch.success":
				return fetchSuccessCounter
			case "revok.change_discoverer.fetch.failed":
				return fetchFailedCounter
			case "revok.change_discoverer.scan.success":
				return scanSuccessCounter
			case "revok.change_discoverer.scan.failed":
				return scanFailedCounter
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

		createCommit("refs/heads/master", remoteRepoPath, "some-file", []byte("credential"), "Initial commit", nil)

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
			scanner,
			repositoryRepository,
			fetchRepository,
			fetchIntervalUpdater,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
		os.RemoveAll(repoToFetchPath)
		os.RemoveAll(remoteRepoPath)
	})

	It("increments the run metric", func() {
		Eventually(runCounter.IncCallCount).Should(Equal(1))
	})

	It("tries to get repositories from the database immediately on start", func() {
		Eventually(repositoryRepository.DueForFetchCallCount).Should(Equal(1))
	})

	It("tries to get repositories from the database on a timer", func() {
		Eventually(repositoryRepository.DueForFetchCallCount).Should(Equal(1))
		Consistently(repositoryRepository.DueForFetchCallCount).Should(Equal(1))

		clock.Increment(interval)

		Eventually(repositoryRepository.DueForFetchCallCount).Should(Equal(2))
		Consistently(repositoryRepository.DueForFetchCallCount).Should(Equal(2))
	})

	Context("when there are repositories to fetch", func() {
		var (
			repo db.Repository
		)

		BeforeEach(func() {
			repo = db.Repository{
				Model: db.Model{
					ID: 42,
				},
				Owner: "some-owner",
				Name:  "some-repo",
				Path:  repoToFetchPath,
			}
			repositoryRepository.DueForFetchReturns([]db.Repository{repo}, nil)
		})

		It("increments the repositories to fetch metric", func() {
			Eventually(reposToFetch.UpdateCallCount).Should(Equal(1))
		})

		It("increments the fetch success metric", func() {
			Eventually(fetchSuccessCounter.IncCallCount).Should(Equal(1))
		})

		Context("when there is an error fetching", func() {
			BeforeEach(func() {
				fakeGitClient := &gitclientfakes.FakeClient{}
				fakeGitClient.FetchReturns(nil, errors.New("an-error"))
				gitClient = fakeGitClient
			})

			It("registers the failed fetch", func() {
				Eventually(repositoryRepository.RegisterFailedFetchCallCount).Should(Equal(1))
			})
		})

		Context("when the remote has changes", func() {
			var results []createCommitResult

			BeforeEach(func() {
				results = []createCommitResult{}

				result := createCommit("refs/heads/master", remoteRepoPath, "some-other-file", []byte("credential"), "second commit", nil)
				results = append(results, result)

				result = createCommit("refs/heads/topicA", remoteRepoPath, "some-file", []byte("credential"), "Initial commit", nil)
				results = append(results, result)
			})

			It("increments the scan success metric", func() {
				Eventually(scanSuccessCounter.IncCallCount).Should(Equal(2))
			})

			It("scans only the changes", func() {
				Eventually(scanner.ScanCallCount).Should(Equal(2)) // 2 new commits

				for i := 0; i < scanner.ScanCallCount(); i++ {
					_, owner, name, _, startSHA, stopSHA := scanner.ScanArgsForCall(i)
					Expect(owner).To(Equal("some-owner"))
					Expect(name).To(Equal("some-repo"))

					for _, result := range results {
						if result.To.String() == startSHA {
							Expect(stopSHA).To(Equal(result.From.String()))
						}
					}
				}
			})

			It("tries to store information in the database about the fetch", func() {
				Eventually(fetchRepository.RegisterFetchCallCount).Should(Equal(1))
				_, fetch := fetchRepository.RegisterFetchArgsForCall(0)
				Expect(fetch.Path).To(Equal(repoToFetchPath))
				Expect(fetch.Repository.ID).To(BeNumerically(">", 0))

				localRepo, err := git.OpenRepository(repoToFetchPath)
				Expect(err).NotTo(HaveOccurred())
				defer localRepo.Free()

				referenceIterator, err := localRepo.NewReferenceIteratorGlob("refs/remotes/origin/*")
				Expect(err).NotTo(HaveOccurred())
				defer referenceIterator.Free()

				expectedChanges := map[string][]*git.Oid{}

				for {
					ref, err := referenceIterator.Next()
					if git.IsErrorCode(err, git.ErrIterOver) {
						break
					}
					Expect(err).NotTo(HaveOccurred())

					if ref.Name() == "refs/remotes/origin/topicA" {
						zeroOid, err := git.NewOid("0000000000000000000000000000000000000000")
						Expect(err).NotTo(HaveOccurred())

						expectedChanges[ref.Name()] = []*git.Oid{zeroOid, ref.Target()}
					} else {
						target, err := localRepo.Lookup(ref.Target())
						Expect(err).NotTo(HaveOccurred())
						defer target.Free()

						targetCommit, err := target.AsCommit()
						Expect(err).NotTo(HaveOccurred())
						defer targetCommit.Free()

						expectedChanges[ref.Name()] = []*git.Oid{targetCommit.ParentId(0), ref.Target()}
					}
				}

				bs, err := json.Marshal(expectedChanges)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetch.Changes).To(Equal(bs))
			})

			It("updates the fetch interval for the repository", func() {
				Eventually(fetchIntervalUpdater.UpdateFetchIntervalCallCount).Should(Equal(1))

				passedRepo := fetchIntervalUpdater.UpdateFetchIntervalArgsForCall(0)
				Expect(passedRepo).To(Equal(&repo))
			})

			Context("when there is an error updating fetch interval", func() {
				BeforeEach(func() {
					fetchIntervalUpdater.UpdateFetchIntervalReturns(errors.New("some-error"))
				})

				It("logs the error", func() {
					Eventually(logger).Should(Say("failed-to-update-fetch-interval"))
				})
			})

			Context("when there is an error scanning a change", func() {
				BeforeEach(func() {
					scanner.ScanStub = func(lager.Logger, string, string, map[git.Oid]struct{}, string, string) error {
						if scanner.ScanCallCount() == 1 {
							return nil
						}

						return errors.New("an-error")
					}
				})

				It("increments the failed scan metric", func() {
					Eventually(scanFailedCounter.IncCallCount).Should(Equal(1))
				})
			})
		})

		Context("when there is an error fetching changes", func() {
			BeforeEach(func() {
				fakeGitClient := &gitclientfakes.FakeClient{}
				fakeGitClient.FetchReturns(nil, errors.New("an-error"))
				gitClient = fakeGitClient
			})

			It("increments the failed fetch metric", func() {
				Eventually(fetchFailedCounter.IncCallCount).Should(Equal(1))
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

			repositoryRepository.DueForFetchStub = func() ([]db.Repository, error) {
				if repositoryRepository.DueForFetchCallCount() == 1 {
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

			createCommit("refs/heads/master", remoteRepoPath, "some-other-file", []byte("credential"), "second commit", nil)

			repositoryRepository.DueForFetchReturns(nil, errors.New("an-error"))
		})

		It("does not increment the repositories to fetch metric", func() {
			Consistently(reposToFetch.UpdateCallCount).Should(BeZero())
		})

		It("does not increment the success metric", func() {
			Consistently(fetchSuccessCounter.IncCallCount).Should(BeZero())
		})

		It("does not do any fetches", func() {
			Consistently(func() string {
				bs, err := ioutil.ReadFile(remoteRefPath)
				Expect(err).NotTo(HaveOccurred())
				return string(bs)
			}).Should(Equal(oldTarget))
		})

		It("does not save any fetches", func() {
			Consistently(fetchRepository.RegisterFetchCallCount).Should(BeZero())
		})
	})
})

package revok_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"gopkg.in/libgit2/git2go.v24"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"

	"context"
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
)

var _ = Describe("ChangeFetcher", func() {
	var (
		logger               *lagertest.TestLogger
		gitFetcherClient     *revokfakes.FakeGitFetchClient
		githubService        *revokfakes.FakeGithubService
		notificationComposer *revokfakes.FakeChangeFetcherNotificationComposer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		fetchRepository      *dbfakes.FakeFetchRepository
		emitter              *metricsfakes.FakeEmitter

		currentFetchID       uint
		fetchedRepositoryIds []uint

		fetchTimer          *metricsfakes.FakeTimer
		fetchSuccessCounter *metricsfakes.FakeCounter
		fetchFailedCounter  *metricsfakes.FakeCounter
		scanSuccessCounter  *metricsfakes.FakeCounter
		scanFailedCounter   *metricsfakes.FakeCounter

		fetcher *revok.ChangeFetcher

		remoteRepoPath  string
		repoToFetchPath string

		repo     db.Repository
		reenable bool
		fetchErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		gitFetcherClient = &revokfakes.FakeGitFetchClient{}
		githubService = &revokfakes.FakeGithubService{}

		notificationComposer = &revokfakes.FakeChangeFetcherNotificationComposer{}

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

		emitter = &metricsfakes.FakeEmitter{}
		fetchSuccessCounter = &metricsfakes.FakeCounter{}
		fetchFailedCounter = &metricsfakes.FakeCounter{}
		scanSuccessCounter = &metricsfakes.FakeCounter{}
		scanFailedCounter = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
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

		var err error
		remoteRepoPath, err = ioutil.TempDir("", "change-fetcher-remote-repo")
		Expect(err).NotTo(HaveOccurred())

		remoteRepo, err := git.InitRepository(remoteRepoPath, false)
		Expect(err).NotTo(HaveOccurred())
		defer remoteRepo.Free()

		createCommit("refs/heads/master", remoteRepoPath, "some-file", []byte("credential"), "Initial commit", nil)

		repoToFetchPath, err = ioutil.TempDir("", "change-fetcher-repo-to-fetch")
		Expect(err).NotTo(HaveOccurred())

		repo = db.Repository{
			Model: db.Model{
				ID: 42,
			},
			Owner:  "some-owner",
			Name:   "some-repo",
			Path:   repoToFetchPath,
			Cloned: true,
		}
		reenable = false

		repositoryRepository.FindReturns(repo, true, nil)
	})

	fetch := func() {
		fetcher = revok.NewChangeFetcher(
			logger,
			gitFetcherClient,
			githubService,
			notificationComposer,
			repositoryRepository,
			fetchRepository,
			emitter,
		)

		fetchErr = fetcher.Fetch(context.Background(), logger, repo.Owner, repo.Name, reenable)
	}

	AfterEach(func() {
		os.RemoveAll(repoToFetchPath)
		os.RemoveAll(remoteRepoPath)
	})

	It("increments the fetch success metric", func() {
		fetch()
		Expect(fetchSuccessCounter.IncCallCount()).To(Equal(1))
	})

	It("finds the repo in the database (to make sure it is up to date)", func() {
		fetch()
		Expect(repositoryRepository.FindCallCount()).To(Equal(1))

		foundOwner, foundName := repositoryRepository.FindArgsForCall(0)
		Expect(foundOwner).To(Equal(repo.Owner))
		Expect(foundName).To(Equal(repo.Name))
	})

	Context("when there is an error loading the repo from the database", func() {
		BeforeEach(func() {
			repositoryRepository.FindReturns(db.Repository{}, false, errors.New("disaster"))

			fetch()
		})

		It("does not try and fetch it", func() {
			Expect(gitFetcherClient.FetchCallCount()).To(BeZero())
		})

		It("errors", func() {
			Expect(fetchErr).To(HaveOccurred())
		})
	})

	Context("when the repository can't be found", func() {
		BeforeEach(func() {
			repositoryRepository.FindReturns(db.Repository{}, false, nil)

			fetch()
		})

		It("does not try and fetch it", func() {
			Expect(gitFetcherClient.FetchCallCount()).To(BeZero())
		})

		It("does not return an error", func() {
			Expect(fetchErr).NotTo(HaveOccurred())
		})

		It("logs", func() {
			Expect(logger).To(Say("skipping-fetch-of-unknown-repo"))
		})
	})

	Context("when there is an error fetching", func() {
		Context("when the repo exists in github", func() {
			BeforeEach(func() {
				gitFetcherClient.FetchReturns(nil, errors.New("an-error"))
				githubService.GetRepoReturns(nil, nil)

				fetch()
			})

			It("registers the failed fetch", func() {
				Expect(repositoryRepository.RegisterFailedFetchCallCount()).To(Equal(1))
			})
		})
		Context("when the repo does not exist or is inaccessible in github", func() {
			BeforeEach(func() {
				gitFetcherClient.FetchReturns(nil, errors.New("an-error"))
				githubService.GetRepoReturns(nil, errors.New("github-error"))

				fetch()
			})

			It("deletes the entry from the database", func() {
				Expect(repositoryRepository.DeleteCallCount()).To(Equal(1))
			})
		})
	})

	Context("when the repository is disabled", func() {
		BeforeEach(func() {
			repo.Disabled = true
			repositoryRepository.FindReturns(repo, true, nil)
		})

		Context("when reenable is true", func() {
			BeforeEach(func() {
				reenable = true
			})

			It("fetches the repository", func() {
				fetch()

				Expect(gitFetcherClient.FetchCallCount()).To(Equal(1))
			})

			It("reenables the repository", func() {
				fetch()

				Expect(repositoryRepository.ReenableCallCount()).To(Equal(1))

				owner, repoName := repositoryRepository.ReenableArgsForCall(0)
				Expect(owner).To(Equal(repo.Owner))
				Expect(repoName).To(Equal(repo.Name))
			})

			It("does not return an error", func() {
				fetch()

				Expect(fetchErr).NotTo(HaveOccurred())
			})

			Context("when re-enabling fails", func() {
				BeforeEach(func() {
					repositoryRepository.ReenableReturns(errors.New("disaster"))
				})

				It("does not fetch", func() {
					fetch()

					Expect(gitFetcherClient.FetchCallCount()).To(BeZero())
				})

				It("returns an error", func() {
					fetch()

					Expect(fetchErr).To(HaveOccurred())
				})
			})
		})

		Context("when reenable is false", func() {
			It("does not try and fetch it", func() {
				fetch()

				Expect(gitFetcherClient.FetchCallCount()).To(BeZero())
			})

			It("does not return an error", func() {
				fetch()

				Expect(fetchErr).NotTo(HaveOccurred())
			})
		})
	})

	Context("when the repository is not cloned yet", func() {
		BeforeEach(func() {
			repo.Cloned = false
			repositoryRepository.FindReturns(repo, true, nil)

			fetch()
		})

		It("does not try and fetch it", func() {
			Expect(gitFetcherClient.FetchCallCount()).To(BeZero())
		})

		It("does not return an error", func() {
			Expect(fetchErr).NotTo(HaveOccurred())
		})
	})

	Context("when the remote has changes", func() {
		var results map[string][]string

		BeforeEach(func() {
			results = make(map[string][]string)
			results["refs/remotes/origin/master"] = []string{
				"something",
				"something else",
			}
			results["refs/remotes/origin/topicA"] = []string{
				"0000000000000000000000000000000000000000",
				"some-sha",
			}

			gitFetcherClient.FetchReturns(results, nil)

		})

		JustBeforeEach(func() {
			fetch()
		})

		It("is successful", func() {
			Expect(fetchErr).NotTo(HaveOccurred())
		})

		It("increments the scan success metric", func() {
			Expect(scanSuccessCounter.IncCallCount()).To(Equal(2))
		})

		It("scans only the changes", func() {

			Expect(notificationComposer.ScanAndNotifyCallCount()).To(Equal(2)) // 2 new commits

			var branches []string
			for i := 0; i < notificationComposer.ScanAndNotifyCallCount(); i++ {
				_, _, owner, name, _, branch, _, _ := notificationComposer.ScanAndNotifyArgsForCall(i)
				Expect(owner).To(Equal("some-owner"))
				Expect(name).To(Equal("some-repo"))

				branches = append(branches, branch)
			}

			Expect(branches).To(ConsistOf("refs/remotes/origin/master", "refs/remotes/origin/topicA"))
		})

		It("tries to store information in the database about the fetch", func() {
			Expect(fetchRepository.RegisterFetchCallCount()).To(Equal(1))
			_, fetch := fetchRepository.RegisterFetchArgsForCall(0)
			Expect(fetch.Path).To(Equal(repoToFetchPath))
			Expect(fetch.Repository.ID).To(BeNumerically(">", 0))

			bs, err := json.Marshal(results)
			Expect(err).NotTo(HaveOccurred())
			Expect(fetch.Changes).To(Equal(bs))
			Expect(fetchSuccessCounter.IncCallCount()).To(Equal(1))
		})

		Context("when there is an error scanning a change", func() {
			BeforeEach(func() {
				notificationComposer.ScanAndNotifyStub = func(context.Context, lager.Logger, string, string, map[string]struct{}, string, string, string) error {
					if notificationComposer.ScanAndNotifyCallCount() == 1 {
						return nil
					}

					return errors.New("an-error")
				}
			})

			It("increments the failed scan metric", func() {
				Expect(scanFailedCounter.IncCallCount()).To(Equal(1))
			})
		})
	})

	Context("when there is an error fetching changes", func() {
		BeforeEach(func() {
			gitFetcherClient.FetchReturns(nil, errors.New("an-error"))
		})

		It("increments the failed fetch metric", func() {
			fetch()

			Expect(fetchFailedCounter.IncCallCount()).To(Equal(1))
		})
	})
})

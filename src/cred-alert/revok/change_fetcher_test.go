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

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	git "gopkg.in/libgit2/git2go.v25"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChangeFetcher", func() {
	var (
		logger               *lagertest.TestLogger
		gitClient            gitclient.Client
		scanner              *revokfakes.FakeScanner
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

		remoteRepoPath  string
		repoToFetchPath string
		repoToFetch     *git.Repository

		fetcher revok.ChangeFetcher

		repo     db.Repository
		fetchErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		gitClient = gitclient.New("private-key-path", "public-key-path")

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

		repoToFetch, err = git.Clone(remoteRepoPath, repoToFetchPath, &git.CloneOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer repoToFetch.Free()

		repo = db.Repository{
			Model: db.Model{
				ID: 42,
			},
			Owner: "some-owner",
			Name:  "some-repo",
			Path:  repoToFetchPath,
		}
		repositoryRepository.FindReturns(repo, nil)
	})

	JustBeforeEach(func() {
		fetcher = revok.NewChangeFetcher(
			logger,
			gitClient,
			scanner,
			repositoryRepository,
			fetchRepository,
			emitter,
		)

		fetchErr = fetcher.Fetch(logger, repo.Owner, repo.Name)
	})

	AfterEach(func() {
		os.RemoveAll(repoToFetchPath)
		os.RemoveAll(remoteRepoPath)
	})

	It("increments the fetch success metric", func() {
		Expect(fetchSuccessCounter.IncCallCount()).To(Equal(1))
	})

	It("finds the repo in the database (to make sure it is up to date)", func() {
		Expect(repositoryRepository.FindCallCount()).To(Equal(1))

		foundOwner, foundName := repositoryRepository.FindArgsForCall(0)
		Expect(foundOwner).To(Equal(repo.Owner))
		Expect(foundName).To(Equal(repo.Name))
	})

	Context("when there is an error loading the repo from the database", func() {
		var fakeGitClient *gitclientfakes.FakeClient

		BeforeEach(func() {
			repositoryRepository.FindReturns(db.Repository{}, errors.New("disaster"))

			fakeGitClient = &gitclientfakes.FakeClient{}
			gitClient = fakeGitClient
		})

		It("does not try and fetch it", func() {
			Expect(fakeGitClient.FetchCallCount()).To(BeZero())
		})

		It("errors", func() {
			Expect(fetchErr).To(HaveOccurred())
		})
	})

	Context("when there is an error fetching", func() {
		BeforeEach(func() {
			fakeGitClient := &gitclientfakes.FakeClient{}
			fakeGitClient.FetchReturns(nil, errors.New("an-error"))
			gitClient = fakeGitClient
		})

		It("registers the failed fetch", func() {
			Expect(repositoryRepository.RegisterFailedFetchCallCount()).To(Equal(1))
		})
	})

	Context("when the repository is disabled", func() {
		var fakeGitClient *gitclientfakes.FakeClient

		BeforeEach(func() {
			fakeGitClient = &gitclientfakes.FakeClient{}
			gitClient = fakeGitClient

			repo.Disabled = true
			repositoryRepository.FindReturns(repo, nil)
		})

		It("does not try an fetch it", func() {
			Expect(fakeGitClient.FetchCallCount()).To(BeZero())
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

		It("is successful", func() {
			Expect(fetchErr).NotTo(HaveOccurred())
		})

		It("increments the scan success metric", func() {
			Expect(scanSuccessCounter.IncCallCount()).To(Equal(2))
		})

		It("scans only the changes", func() {
			Expect(scanner.ScanCallCount()).To(Equal(2)) // 2 new commits

			var branches []string
			for i := 0; i < scanner.ScanCallCount(); i++ {
				_, owner, name, _, branch, startSHA, stopSHA := scanner.ScanArgsForCall(i)
				Expect(owner).To(Equal("some-owner"))
				Expect(name).To(Equal("some-repo"))

				branches = append(branches, branch)
				for _, result := range results {
					if result.To.String() == startSHA {
						Expect(stopSHA).To(Equal(result.From.String()))
					}
				}
			}

			Expect(branches).To(ConsistOf("refs/remotes/origin/master", "refs/remotes/origin/topicA"))
		})

		It("tries to store information in the database about the fetch", func() {
			Expect(fetchRepository.RegisterFetchCallCount()).To(Equal(1))
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

		Context("when there is an error scanning a change", func() {
			BeforeEach(func() {
				scanner.ScanStub = func(lager.Logger, string, string, map[git.Oid]struct{}, string, string, string) error {
					if scanner.ScanCallCount() == 1 {
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
			fakeGitClient := &gitclientfakes.FakeClient{}
			fakeGitClient.FetchReturns(nil, errors.New("an-error"))
			gitClient = fakeGitClient
		})

		It("increments the failed fetch metric", func() {
			Expect(fetchFailedCounter.IncCallCount()).To(Equal(1))
		})
	})
})

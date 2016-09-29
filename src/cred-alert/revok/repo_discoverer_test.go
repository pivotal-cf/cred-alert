package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepoDiscoverer", func() {
	var (
		logger               *lagertest.TestLogger
		clock                *fakeclock.FakeClock
		interval             time.Duration
		cloneMsgCh           chan revok.CloneMsg
		ghClient             *revokfakes.FakeGitHubClient
		workdir              string
		repositoryRepository *dbfakes.FakeRepositoryRepository
		currentRepositoryID  uint

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		clock = fakeclock.NewFakeClock(time.Now())
		interval = 1 * time.Hour

		cloneMsgCh = make(chan revok.CloneMsg, 10)

		var err error
		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())

		ghClient = &revokfakes.FakeGitHubClient{}
		ghClient.ListRepositoriesStub = func(lager.Logger) ([]revok.GitHubRepository, error) {
			if ghClient.ListRepositoriesCallCount() == 1 {
				return []revok.GitHubRepository{boshSampleReleaseRepository}, nil
			}

			return []revok.GitHubRepository{
				boshSampleReleaseRepository,
				cfMessageBusRepository,
			}, nil
		}

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		currentRepositoryID = 0
		repositoryRepository.CreateStub = func(r *db.Repository) error {
			currentRepositoryID++
			r.ID = currentRepositoryID
			return nil
		}

		repositoryRepository.AllStub = func() ([]db.Repository, error) {
			if repositoryRepository.AllCallCount() == 1 {
				return []db.Repository{}, nil
			}

			return []db.Repository{
				{
					Name:  "bosh-sample-release",
					Owner: "cloudfoundry",
				},
			}, nil
		}
	})

	JustBeforeEach(func() {
		runner = revok.NewRepoDiscoverer(
			logger,
			workdir,
			cloneMsgCh,
			ghClient,
			clock,
			interval,
			repositoryRepository,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
		os.RemoveAll(workdir)
	})

	It("does work immediately on start", func() {
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(1))
	})

	It("does work once per interval", func() {
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(1))
		Consistently(ghClient.ListRepositoriesCallCount).Should(Equal(1))
		clock.Increment(interval)
		Eventually(ghClient.ListRepositoriesCallCount).Should(Equal(2))
		Consistently(ghClient.ListRepositoriesCallCount).Should(Equal(2))
	})

	It("puts a job on the jobs channel", func() {
		var cloneMsg revok.CloneMsg
		Eventually(cloneMsgCh).Should(Receive(&cloneMsg))
		Expect(cloneMsg.URL).To(Equal("git@github.com:cloudfoundry/bosh-sample-release.git"))
	})

	It("tries to store only new repos in the database", func() {
		Eventually(repositoryRepository.AllCallCount).Should(Equal(1))
		Eventually(repositoryRepository.CreateCallCount).Should(Equal(1))
		repo := repositoryRepository.CreateArgsForCall(0)
		Expect(repo.Owner).To(Equal("cloudfoundry"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("bosh-sample-release"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("git@github.com:cloudfoundry/bosh-sample-release.git"))
		Expect(repo.Private).To(BeFalse())
		Expect(repo.DefaultBranch).To(Equal("master"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(boshSampleReleaseRepositoryJSON)))

		clock.Increment(interval)

		Eventually(repositoryRepository.CreateCallCount).Should(Equal(2))
		repo = repositoryRepository.CreateArgsForCall(1)
		Expect(repo.Owner).To(Equal("cloudfoundry"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("cf-message-bus"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("git@github.com:cloudfoundry/cf-message-bus.git"))
		Expect(repo.Private).To(BeFalse())
		Expect(repo.DefaultBranch).To(Equal("master"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(cfMessageBusJSON)))

		Consistently(repositoryRepository.CreateCallCount).Should(Equal(2))
	})
})

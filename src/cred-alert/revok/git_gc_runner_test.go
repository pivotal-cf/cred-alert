package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("GitGCRunner", func() {
	var (
		runner  ifrit.Runner
		process ifrit.Process

		fakeClock       *fakeclock.FakeClock
		fakeRepoRepo    *dbfakes.FakeRepositoryRepository
		fakeGitGCClient *revokfakes.FakeGitGCClient
		logger          *lagertest.TestLogger

		retryInterval time.Duration

		repositories []db.Repository
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeRepoRepo = &dbfakes.FakeRepositoryRepository{}
		fakeGitGCClient = &revokfakes.FakeGitGCClient{}
		logger = lagertest.NewTestLogger("git-gc")

		retryInterval = 24 * time.Hour

		repositories = []db.Repository{
			{Path: "/path/to/repo1", Cloned: true},
			{Path: "/path/to/repo2", Cloned: true},
			{Path: "/path/to/repo3", Cloned: true},
			{Path: "/path/to/repo4", Cloned: false},
		}

		fakeRepoRepo.AllReturns(repositories, nil)

		runner = revok.NewGitGCRunner(
			logger,
			fakeClock,
			fakeRepoRepo,
			fakeGitGCClient,
			retryInterval,
		)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Kill(process)
	})

	It("fetches the list of repositories and runs git gc on all of them", func() {
		Eventually(fakeRepoRepo.AllCallCount).Should(Equal(1))
		Eventually(fakeGitGCClient.GCCallCount).Should(Equal(3))

		repoPath := fakeGitGCClient.GCArgsForCall(0)
		Expect(repoPath).To(Equal(repositories[0].Path))

		repoPath = fakeGitGCClient.GCArgsForCall(1)
		Expect(repoPath).To(Equal(repositories[1].Path))

		repoPath = fakeGitGCClient.GCArgsForCall(2)
		Expect(repoPath).To(Equal(repositories[2].Path))
	})

	Context("when the retry interval has passed", func() {
		It("runs the git gc on all the repos again", func() {
			Eventually(fakeRepoRepo.AllCallCount).Should(Equal(1))
			Eventually(fakeGitGCClient.GCCallCount).Should(Equal(3))

			repoPath := fakeGitGCClient.GCArgsForCall(0)
			Expect(repoPath).To(Equal(repositories[0].Path))

			repoPath = fakeGitGCClient.GCArgsForCall(1)
			Expect(repoPath).To(Equal(repositories[1].Path))

			repoPath = fakeGitGCClient.GCArgsForCall(2)
			Expect(repoPath).To(Equal(repositories[2].Path))

			fakeClock.WaitForWatcherAndIncrement(retryInterval)

			Eventually(fakeRepoRepo.AllCallCount).Should(Equal(2))
			Eventually(fakeGitGCClient.GCCallCount).Should(Equal(6))

			repoPath = fakeGitGCClient.GCArgsForCall(3)
			Expect(repoPath).To(Equal(repositories[0].Path))

			repoPath = fakeGitGCClient.GCArgsForCall(4)
			Expect(repoPath).To(Equal(repositories[1].Path))

			repoPath = fakeGitGCClient.GCArgsForCall(5)
			Expect(repoPath).To(Equal(repositories[2].Path))
		})
	})

	Context("when fetching the repositories fails", func() {
		BeforeEach(func() {
			fakeRepoRepo.AllReturns(nil, errors.New("boom"))
		})

		It("logs the failure", func() {
			Eventually(logger).Should(gbytes.Say("failed-fetching-repos"))
			Consistently(process.Wait()).ShouldNot(Receive())
		})
	})

	Context("when running git-gc fails", func() {
		BeforeEach(func() {
			fakeGitGCClient.GCReturns(errors.New("boom"))
		})

		It("logs an continues to run git gc on the other repos", func() {
			Eventually(logger).Should(gbytes.Say("failed-running-git-gc"))
			Eventually(fakeGitGCClient.GCCallCount).Should(Equal(3))
			Consistently(process.Wait()).ShouldNot(Receive())
		})
	})

	Context("when signalled", func() {
		It("exits", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
			process.Signal(os.Kill)
			Eventually(process.Wait()).Should(Receive())
		})
	})
})

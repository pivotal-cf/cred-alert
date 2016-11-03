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
		ghClient.ListOrganizationsStub = func(lager.Logger) ([]revok.GitHubOrganization, error) {
			return []revok.GitHubOrganization{
				{
					Name: "some-org",
				},
				{
					Name: "some-other-org",
				},
			}, nil
		}

		ghClient.ListRepositoriesByOrgStub = func(l lager.Logger, orgName string) ([]revok.GitHubRepository, error) {
			switch ghClient.ListOrganizationsCallCount() {
			case 1:
				switch orgName {
				case "some-org":
					return []revok.GitHubRepository{org1Repo1}, nil
				case "some-other-org":
					return []revok.GitHubRepository{org2Repo1, org2Repo2}, nil
				}
			case 2:
				switch orgName {
				case "some-org":
					return []revok.GitHubRepository{org1Repo1, org1Repo2}, nil
				case "some-other-org":
					return []revok.GitHubRepository{org2Repo1, org2Repo2}, nil
				}
			}

			panic("need more stubs")
		}

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		currentRepositoryID = 0
		repositoryRepository.CreateStub = func(r *db.Repository) error {
			currentRepositoryID++
			r.ID = currentRepositoryID
			return nil
		}

		repositoryRepository.AllStub = func() ([]db.Repository, error) {
			switch repositoryRepository.AllCallCount() {
			case 1:
				return []db.Repository{}, nil
			case 2:
				return []db.Repository{
					{
						Owner: "some-org",
						Name:  "org-1-repo-1",
					},
					{
						Owner: "some-other-org",
						Name:  "org-2-repo-1",
					},
					{
						Owner: "some-other-org",
						Name:  "org-2-repo-2",
					},
				}, nil
			}

			panic("need more stubs")
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
		os.RemoveAll(workdir)
	})

	It("does work immediately on start", func() {
		Eventually(ghClient.ListOrganizationsCallCount).Should(Equal(1))
	})

	It("does work once per interval", func() {
		Eventually(ghClient.ListOrganizationsCallCount).Should(Equal(1))
		Consistently(ghClient.ListOrganizationsCallCount).Should(Equal(1))
		clock.Increment(interval)
		Eventually(ghClient.ListOrganizationsCallCount).Should(Equal(2))
		Consistently(ghClient.ListOrganizationsCallCount).Should(Equal(2))
	})

	It("puts a job on the jobs channel for each new repository", func() {
		var cloneMsg revok.CloneMsg

		Eventually(cloneMsgCh).Should(Receive(&cloneMsg))
		Expect(cloneMsg.Owner).To(Equal("some-org"))
		Expect(cloneMsg.URL).To(Equal("org-1-repo-1-ssh-url"))

		Eventually(cloneMsgCh).Should(Receive(&cloneMsg))
		Expect(cloneMsg.Owner).To(Equal("some-other-org"))
		Expect(cloneMsg.URL).To(Equal("org-2-repo-1-ssh-url"))

		Eventually(cloneMsgCh).Should(Receive(&cloneMsg))
		Expect(cloneMsg.Owner).To(Equal("some-other-org"))
		Expect(cloneMsg.URL).To(Equal("org-2-repo-2-ssh-url"))

		clock.Increment(interval)

		Eventually(cloneMsgCh).Should(Receive(&cloneMsg))
		Expect(cloneMsg.Owner).To(Equal("some-org"))
		Expect(cloneMsg.URL).To(Equal("org-1-repo-2-ssh-url"))
	})

	It("tries to store only new repos in the database", func() {
		Eventually(repositoryRepository.CreateCallCount).Should(Equal(3))
		repo := repositoryRepository.CreateArgsForCall(0)
		Expect(repo.Owner).To(Equal("some-org"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("org-1-repo-1"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("org-1-repo-1-ssh-url"))
		Expect(repo.Private).To(BeTrue())
		Expect(repo.DefaultBranch).To(Equal("org-1-repo-1-branch"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(`{"some-key":"some-value"}`)))

		// org-1-repo-2 comes later
		repo = repositoryRepository.CreateArgsForCall(1)
		Expect(repo.Owner).To(Equal("some-other-org"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("org-2-repo-1"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("org-2-repo-1-ssh-url"))
		Expect(repo.Private).To(BeTrue())
		Expect(repo.DefaultBranch).To(Equal("org-2-repo-1-branch"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(`{"some-key":"some-value"}`)))

		repo = repositoryRepository.CreateArgsForCall(2)
		Expect(repo.Owner).To(Equal("some-other-org"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("org-2-repo-2"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("org-2-repo-2-ssh-url"))
		Expect(repo.Private).To(BeTrue())
		Expect(repo.DefaultBranch).To(Equal("org-2-repo-2-branch"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(`{"some-key":"some-value"}`)))

		clock.Increment(interval)

		Eventually(repositoryRepository.CreateCallCount).Should(Equal(4))

		repo = repositoryRepository.CreateArgsForCall(3)
		Expect(repo.Owner).To(Equal("some-org"))
		Expect(repo.Cloned).To(BeFalse())
		Expect(repo.Name).To(Equal("org-1-repo-2"))
		Expect(repo.Path).To(Equal(""))
		Expect(repo.SSHURL).To(Equal("org-1-repo-2-ssh-url"))
		Expect(repo.Private).To(BeTrue())
		Expect(repo.DefaultBranch).To(Equal("org-1-repo-2-branch"))
		Expect(repo.RawJSON).To(MatchJSON([]byte(`{"some-key":"some-value"}`)))
	})
})

package revok_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/revok"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("HeadCredentialCounter", func() {
	var (
		logger               *lagertest.TestLogger
		repositoryRepository *dbfakes.FakeRepositoryRepository
		clock                *fakeclock.FakeClock
		interval             time.Duration
		gitClient            *gitclientfakes.FakeClient
		sniffer              *snifffakes.FakeSniffer

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		clock = fakeclock.NewFakeClock(time.Now())
		interval = 1 * time.Hour
		gitClient = &gitclientfakes.FakeClient{}

		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			var start, end int
			for s.Scan(logger) {
				start += 1
				end += 2
				line := s.Line(logger)
				if strings.Contains(string(line.Content), "credential") {
					h(l, scanners.Violation{
						Line:  *line,
						Start: start,
						End:   end,
					})
				}
			}

			return nil
		}
	})

	JustBeforeEach(func() {
		runner = revok.NewHeadCredentialCounter(
			logger,
			repositoryRepository,
			clock,
			interval,
			gitClient,
			sniffer,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("tries to get repositories from the database immediately on start", func() {
		Eventually(repositoryRepository.AllCallCount).Should(Equal(1))
	})

	It("does work once per interval", func() {
		Eventually(repositoryRepository.AllCallCount).Should(Equal(1))
		Consistently(repositoryRepository.AllCallCount).Should(Equal(1))
		clock.Increment(interval)
		Eventually(repositoryRepository.AllCallCount).Should(Equal(2))
		Consistently(repositoryRepository.AllCallCount).Should(Equal(2))
	})

	Context("when there are repositories in the database", func() {
		var repo1, repo2 db.Repository

		BeforeEach(func() {
			repo1 = db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				Path:          "some-path",
				DefaultBranch: "some-branch",
			}
			repo2 = db.Repository{
				Name:          "some-other-repo",
				Owner:         "some-other-owner",
				Path:          "some-other-path",
				DefaultBranch: "some-other-branch",
			}

			repositoryRepository.AllReturns([]db.Repository{repo1, repo2}, nil)
		})

		It("tries to get the credential counts for each repository", func() {
			Eventually(gitClient.BranchCredentialCountsCallCount).Should(Equal(2))

			_, path, _ := gitClient.BranchCredentialCountsArgsForCall(0)
			Expect(path).To(Equal("some-path"))

			_, path, _ = gitClient.BranchCredentialCountsArgsForCall(1)
			Expect(path).To(Equal("some-other-path"))
		})

		Context("when there are credentials for the repository", func() {
			BeforeEach(func() {
				gitClient.BranchCredentialCountsStub = func(l lager.Logger, path string, s sniff.Sniffer) (map[string]uint, error) {
					defer GinkgoRecover()

					switch path {
					case "some-path":
						return map[string]uint{
							"branch-1": 1,
							"branch-2": 2,
						}, nil
					case "some-other-path":
						return map[string]uint{
							"branch-3": 3,
							"branch-4": 4,
						}, nil
					default:
						panic(fmt.Sprintf("no stub for '%s'", path))
					}
				}
			})

			It("tries to store the count of credentials in the repository in the database", func() {
				Eventually(repositoryRepository.UpdateCredentialCountCallCount).Should(Equal(2))
				repo, counts := repositoryRepository.UpdateCredentialCountArgsForCall(0)
				Expect(repo).To(Equal(&repo1))
				Expect(counts).To(Equal(map[string]uint{
					"branch-1": 1,
					"branch-2": 2,
				}))

				repo, counts = repositoryRepository.UpdateCredentialCountArgsForCall(1)
				Expect(repo).To(Equal(&repo2))
				Expect(counts).To(Equal(map[string]uint{
					"branch-3": 3,
					"branch-4": 4,
				}))
			})
		})

		Context("when it is signaled in the middle of work", func() {
			BeforeEach(func() {
				gitClient.BranchTargetsReturns(map[string]string{"some-branch": "some-target"}, nil)

				var repositories []db.Repository
				for i := 0; i < 50; i++ {
					repositories = append(repositories, db.Repository{
						Path: fmt.Sprintf("some-path-%d", i),
					})
				}

				repositoryRepository.AllReturns(repositories, nil)
			})

			It("exits gracefully and quickly", func() {
				process.Signal(os.Interrupt)

				// give the counts time to converge; they should eventually be the same
				Eventually(repositoryRepository.UpdateCredentialCountCallCount, 2*time.Second).Should(BeNumerically("~", gitClient.BranchCredentialCountsCallCount(), 1))
			})
		})

		Context("when getting blobs returns an error", func() {
			BeforeEach(func() {
				gitClient.BranchCredentialCountsStub = func(l lager.Logger, path string, s sniff.Sniffer) (map[string]uint, error) {
					defer GinkgoRecover()

					switch path {
					case "some-path":
						return nil, errors.New("an-error")
					case "some-other-path":
						return map[string]uint{
							"branch-3": 3,
							"branch-4": 4,
						}, nil
					default:
						panic(fmt.Sprintf("no stub for '%s'", path))
					}
				}
			})

			It("continues to the next repository", func() {
				Eventually(repositoryRepository.UpdateCredentialCountCallCount).Should(Equal(1))
				repo, counts := repositoryRepository.UpdateCredentialCountArgsForCall(0)
				Expect(repo).To(Equal(&repo2))
				Expect(counts).To(Equal(map[string]uint{
					"branch-3": 3,
					"branch-4": 4,
				}))
			})
		})

		Context("when getting blobs returns gitclient.ErrInterrupted", func() {
			BeforeEach(func() {
				gitClient.BranchCredentialCountsStub = func(l lager.Logger, path string, s sniff.Sniffer) (map[string]uint, error) {
					defer GinkgoRecover()
					return nil, gitclient.ErrInterrupted
				}
			})

			It("returns immediately", func() {
				Consistently(repositoryRepository.UpdateCredentialCountCallCount).Should(BeZero())
			})
		})
	})
})

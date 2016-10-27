package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/revok"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
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
		gitClient.AllBlobsForRefStub = func(path string, ref string, w *io.PipeWriter) error {
			w.Close()
			return nil
		}

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

		It("tries to get the blobs for each repository", func() {
			Eventually(gitClient.AllBlobsForRefCallCount).Should(Equal(2))
			path, ref, writer := gitClient.AllBlobsForRefArgsForCall(0)
			Expect(path).To(Equal("some-path"))
			Expect(ref).To(Equal("refs/remotes/origin/some-branch"))
			Expect(writer).To(BeAssignableToTypeOf(&io.PipeWriter{}))

			path, ref, writer = gitClient.AllBlobsForRefArgsForCall(1)
			Expect(path).To(Equal("some-other-path"))
			Expect(ref).To(Equal("refs/remotes/origin/some-other-branch"))
			Expect(writer).To(BeAssignableToTypeOf(&io.PipeWriter{}))
		})

		Context("when there are blobs for the repository", func() {
			BeforeEach(func() {
				gitClient.AllBlobsForRefStub = func(path string, ref string, w *io.PipeWriter) error {
					switch ref {
					case "refs/remotes/origin/some-branch":
						w.Write([]byte("credential\n"))
						w.Write([]byte("credential\n"))
						w.Close()
					case "refs/remotes/origin/some-other-branch":
						w.Write([]byte("credential\n"))
						w.Close()
					default:
						panic(fmt.Sprintf("no stub for '%s'", ref))
					}

					return nil
				}
			})

			It("tries to store the count of credentials in the repository in the database", func() {
				Eventually(repositoryRepository.UpdateCredentialCountCallCount).Should(Equal(2))
				repo, count := repositoryRepository.UpdateCredentialCountArgsForCall(0)
				Expect(repo).To(Equal(&repo1))
				Expect(count).To(Equal(uint(2)))

				repo, count = repositoryRepository.UpdateCredentialCountArgsForCall(1)
				Expect(repo).To(Equal(&repo2))
				Expect(count).To(Equal(uint(1)))
			})
		})

		Context("when it is signaled in the middle of work", func() {
			BeforeEach(func() {
				var repositories []db.Repository
				for i := 0; i < 50; i++ {
					repositories = append(repositories, db.Repository{
						Path:          fmt.Sprintf("some-path-%d", i),
						DefaultBranch: "some-branch",
					})
				}

				repositoryRepository.AllReturns(repositories, nil)

				gitClient.AllBlobsForRefStub = func(path string, ref string, w *io.PipeWriter) error {
					w.Write([]byte("credential\n"))
					w.Close()

					return nil
				}
			})

			It("exits gracefully and quickly", func() {
				process.Signal(os.Interrupt)

				// give the counts time to converge; they should eventually be the same
				Eventually(repositoryRepository.UpdateCredentialCountCallCount, 2*time.Second).Should(BeNumerically("~", gitClient.AllBlobsForRefCallCount(), 1))
			})
		})

		Context("when getting blobs returns an error", func() {
			BeforeEach(func() {
				gitClient.AllBlobsForRefStub = func(path string, ref string, w *io.PipeWriter) error {
					switch ref {
					case "refs/remotes/origin/some-branch":
						err := errors.New("an-error")
						w.CloseWithError(err)
						return err
					case "refs/remotes/origin/some-other-branch":
						w.Write([]byte("credential\n"))
						w.Close()
						return nil
					default:
						panic(fmt.Sprintf("no stub for '%s'", ref))
					}
				}
			})

			It("continues to the next repository", func() {
				Eventually(repositoryRepository.UpdateCredentialCountCallCount).Should(Equal(1))
				repo, count := repositoryRepository.UpdateCredentialCountArgsForCall(0)
				Expect(repo).To(Equal(&repo2))
				Expect(count).To(Equal(uint(1)))
			})
		})
	})
})

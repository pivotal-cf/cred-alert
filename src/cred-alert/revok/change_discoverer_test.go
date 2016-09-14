package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
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
	"os"
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
		workdir              string
		gitClient            *gitclientfakes.FakeClient
		clock                *fakeclock.FakeClock
		interval             time.Duration
		repositoryRepository *dbfakes.FakeRepositoryRepository
		fetchRepository      *dbfakes.FakeFetchRepository
		scanRepository       *dbfakes.FakeScanRepository
		emitter              *metricsfakes.FakeEmitter
		sniffer              *snifffakes.FakeSniffer

		firstScan      *dbfakes.FakeActiveScan
		secondScan     *dbfakes.FakeActiveScan
		currentFetchID uint
		successMetric  *metricsfakes.FakeCounter
		failedMetric   *metricsfakes.FakeCounter

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		gitClient = &gitclientfakes.FakeClient{}
		clock = fakeclock.NewFakeClock(time.Now())
		interval = 30 * time.Minute

		var err error
		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		scanRepository = &dbfakes.FakeScanRepository{}
		firstScan = &dbfakes.FakeActiveScan{}
		secondScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartStub = func(lager.Logger, string, *db.Repository, *db.Fetch) db.ActiveScan {
			if scanRepository.StartCallCount() == 1 {
				return firstScan
			}
			return secondScan
		}

		currentFetchID = 0
		fetchRepository = &dbfakes.FakeFetchRepository{}
		fetchRepository.SaveFetchStub = func(l lager.Logger, f *db.Fetch) error {
			currentFetchID++
			f.ID = currentFetchID
			return nil
		}

		emitter = &metricsfakes.FakeEmitter{}
		successMetric = &metricsfakes.FakeCounter{}
		failedMetric = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.success_jobs":
				return successMetric
			case "revok.failed_jobs":
				return failedMetric
			}
			return &metricsfakes.FakeCounter{}
		}
	})

	JustBeforeEach(func() {
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

		runner = revok.NewChangeDiscoverer(
			logger,
			workdir,
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
		os.RemoveAll(workdir)
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
					Path:  "some-path",
				},
			}, nil)
		})

		It("fetches updates for the repo", func() {
			Eventually(gitClient.FetchCallCount).Should(Equal(1))
			Expect(gitClient.FetchArgsForCall(0)).To(Equal("some-path"))
		})

		Context("when the remote has changes", func() {
			var (
				oldOid1 *git.Oid
				newOid1 *git.Oid
				oldOid2 *git.Oid
				newOid2 *git.Oid
				changes map[string][]*git.Oid
			)

			BeforeEach(func() {
				var err error
				oldOid1, err = git.NewOid("fce98866a7d559757a0a501aa548e244a46ad00a")
				Expect(err).NotTo(HaveOccurred())
				newOid1, err = git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
				Expect(err).NotTo(HaveOccurred())
				oldOid2, err = git.NewOid("7257894438275f68380aa6d75015ef7a0ca6757b")
				Expect(err).NotTo(HaveOccurred())
				newOid2, err = git.NewOid("53fc72ccf2ef176a02169aeebf5c8427861e9b0e")
				Expect(err).NotTo(HaveOccurred())

				changes = map[string][]*git.Oid{
					"refs/remotes/origin/master":  {oldOid1, newOid1},
					"refs/remotes/origin/develop": {oldOid2, newOid2},
				}

				gitClient.FetchReturns(changes, nil)

				gitClient.DiffStub = func(repositoryPath string, a, b *git.Oid) (string, error) {
					if gitClient.DiffCallCount() == 1 {
						return `diff --git a/stuff.txt b/stuff.txt
	index f2e4113..fa5a232 100644
	--- a/stuff.txt
	+++ b/stuff.txt
	@@ -1 +1,2 @@
	-old
	+credential
	+something-else`, nil
					}

					return `--git a/stuff.txt b/stuff.txt
	index fa5a232..1e13fe8 100644
	--- a/stuff.txt
	+++ b/stuff.txt
	@@ -1,2 +1 @@
	-old
	-content
	+credential`, nil
				}
			})

			It("does a diff scan on the changes", func() {
				Eventually(gitClient.DiffCallCount).Should(Equal(2))

				// for synchronizing the unordered map returned by Fetch
				expectedOids := map[string][]*git.Oid{
					oldOid1.String(): []*git.Oid{oldOid1, newOid1},
					oldOid2.String(): []*git.Oid{oldOid2, newOid2},
				}

				actualOids := map[string][]*git.Oid{}

				dest, a, b := gitClient.DiffArgsForCall(0)
				Expect(dest).To(Equal("some-path"))
				actualOids[a.String()] = []*git.Oid{a, b}

				// this is the only way to detect anything was scanned
				Expect(firstScan.RecordCredentialCallCount()).To(Equal(1))

				dest, c, d := gitClient.DiffArgsForCall(1)
				Expect(dest).To(Equal("some-path"))
				actualOids[c.String()] = []*git.Oid{c, d}

				Expect(actualOids).To(Equal(expectedOids))

				Expect(secondScan.RecordCredentialCallCount()).To(Equal(1))
			})

			It("tries to store information in the database about the fetch", func() {
				Eventually(fetchRepository.SaveFetchCallCount).Should(Equal(1))
				_, fetch := fetchRepository.SaveFetchArgsForCall(0)
				Expect(fetch.Path).To(Equal("some-path"))
				bs, err := json.Marshal(changes)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetch.Changes).To(Equal(bs))
				Expect(fetch.Repository.ID).To(BeNumerically(">", 0))
			})

			It("tries to store information in the database about found credentials", func() {
				Eventually(scanRepository.StartCallCount).Should(Equal(2))
				_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
				Expect(scanType).To(Equal("diff-scan"))
				Expect(repository.ID).To(BeNumerically("==", 42))
				Expect(fetch.ID).To(Equal(currentFetchID))

				Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
				Eventually(firstScan.FinishCallCount).Should(Equal(1))

				Eventually(secondScan.RecordCredentialCallCount).Should(Equal(1))
				Eventually(secondScan.FinishCallCount).Should(Equal(1))
			})

			It("increments the success metric", func() {
				Eventually(successMetric.IncCallCount).Should(Equal(2))
			})

			Context("when there is an error saving the fetch to the database", func() {
				BeforeEach(func() {
					fetchRepository.SaveFetchReturns(errors.New("an-error"))
				})

				It("does not try to diff anything", func() {
					Consistently(gitClient.DiffCallCount).Should(BeZero())
				})
			})

			XIt("it does a message scan on the changes", func() {
			})

			XContext("when there is an error getting the diff for the changes", func() {
				BeforeEach(func() {
					gitClient.DiffStub = func(dest string, a *git.Oid, b *git.Oid) (string, error) {
						if gitClient.DiffCallCount() == 1 {
							return "", errors.New("an-error")
						}

						return `--git a/stuff.txt b/stuff.txt
	index fa5a232..1e13fe8 100644
	--- a/stuff.txt
	+++ b/stuff.txt
	@@ -1,2 +1 @@
	-old
	-content
	+credential`, nil
					}
				})

				XIt("increments a metric which doesn't exist yet", func() {
				})
			})

			Context("when there is an error storing credentials in the database", func() {
				BeforeEach(func() {
					firstScan.FinishReturns(errors.New("an-error"))
				})

				It("increments the failed metric", func() {
					Eventually(firstScan.FinishCallCount).Should(Equal(1))
					Expect(failedMetric.IncCallCount()).To(Equal(1))
				})
			})
		})
	})

})

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
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cloner", func() {
	var (
		workdir              string
		workCh               chan revok.CloneMsg
		logger               *lagertest.TestLogger
		gitClient            *gitclientfakes.FakeClient
		sniffer              *snifffakes.FakeSniffer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		scanRepository       *dbfakes.FakeScanRepository
		emitter              *metricsfakes.FakeEmitter

		firstScan     *dbfakes.FakeActiveScan
		secondScan    *dbfakes.FakeActiveScan
		successMetric *metricsfakes.FakeCounter
		failedMetric  *metricsfakes.FakeCounter

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		workCh = make(chan revok.CloneMsg, 10)
		gitClient = &gitclientfakes.FakeClient{}
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		repositoryRepository.FindReturns(db.Repository{
			Model: db.Model{
				ID: 42,
			},
		}, nil)

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

		var err error
		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(filepath.Join(workdir, "some-owner", "some-repo"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(workdir, "some-owner", "some-repo", "some-file"), []byte("credential"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			for s.Scan(logger) {
				line := s.Line(logger)
				if strings.Contains(string(line.Content), "credential") {
					h(l, scanners.Violation{Line: *line})
				}
			}

			return nil
		}

		runner = revok.NewCloner(
			logger,
			workdir,
			workCh,
			gitClient,
			sniffer,
			repositoryRepository,
			scanRepository,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		os.RemoveAll(workdir)
	})

	Context("when there is a message on the clone message channel", func() {
		BeforeEach(func() {
			workCh <- revok.CloneMsg{
				URL:        "some-url",
				Repository: "some-repo",
				Owner:      "some-owner",
			}
		})

		It("tries to clone when it receives a message", func() {
			Eventually(gitClient.CloneCallCount).Should(Equal(1))
			url, dest := gitClient.CloneArgsForCall(0)
			Expect(url).To(Equal("some-url"))
			Expect(dest).To(Equal(filepath.Join(workdir, "some-owner", "some-repo")))
		})

		It("updates the repository in the database", func() {
			Eventually(repositoryRepository.MarkAsClonedCallCount).Should(Equal(1))
		})

		It("finds the repository in the database", func() {
			Eventually(repositoryRepository.FindCallCount).Should(Equal(1))
			owner, name := repositoryRepository.FindArgsForCall(0)
			Expect(owner).To(Equal("some-owner"))
			Expect(name).To(Equal("some-repo"))
		})

		It("scans each file in the default branch of the cloned repo", func() {
			Eventually(sniffer.SniffCallCount).Should(Equal(1))
		})

		It("tries to store information in the database about found credentials", func() {
			Eventually(scanRepository.StartCallCount).Should(Equal(1))
			_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
			Expect(scanType).To(Equal("dir-scan"))
			Expect(repository.ID).To(BeNumerically("==", 42))
			Expect(fetch).To(BeNil())

			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
			Eventually(firstScan.FinishCallCount).Should(Equal(1))
		})

		Context("when cloning fails", func() {
			BeforeEach(func() {
				gitClient.CloneStub = func(url, dest string) error {
					err := os.MkdirAll(dest, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					return errors.New("an-error")
				}
			})

			It("cleans up the failed clone destination, if any", func() {
				Eventually(gitClient.CloneCallCount).Should(Equal(1))
				_, dest := gitClient.CloneArgsForCall(0)
				Eventually(dest).ShouldNot(BeADirectory())
			})

			It("does not try to scan", func() {
				Consistently(scanRepository.StartCallCount).Should(BeZero())
			})

			It("does not mark the repository as having been cloned", func() {
				Consistently(repositoryRepository.MarkAsClonedCallCount).Should(BeZero())
			})
		})

		Context("when marking the repository as cloned fails", func() {
			BeforeEach(func() {
				repositoryRepository.MarkAsClonedReturns(errors.New("an-error"))
			})

			It("does not try to scan", func() {
				Consistently(scanRepository.StartCallCount).Should(BeZero())
			})
		})

		Context("when finding the repository fails", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
			})

			It("does not try to scan", func() {
				Consistently(scanRepository.StartCallCount).Should(BeZero())
			})
		})
	})
})

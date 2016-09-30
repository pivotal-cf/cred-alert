package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	git "github.com/libgit2/git2go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scanner", func() {
	var (
		logger               *lagertest.TestLogger
		gitClient            gitclient.Client
		sniffer              *snifffakes.FakeSniffer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		scanRepository       *dbfakes.FakeScanRepository
		emitter              *metricsfakes.FakeEmitter
		scanner              revok.Scanner

		firstScan      *dbfakes.FakeActiveScan
		secondScan     *dbfakes.FakeActiveScan
		successMetric  *metricsfakes.FakeCounter
		failedMetric   *metricsfakes.FakeCounter
		baseRepoPath   string
		repoToScanPath string
		head           *git.Reference
	)

	BeforeEach(func() {
		var err error
		baseRepoPath, err = ioutil.TempDir("", "revok-test-base-repo")
		Expect(err).NotTo(HaveOccurred())

		baseRepo, err := git.InitRepository(baseRepoPath, false)
		Expect(err).NotTo(HaveOccurred())
		defer baseRepo.Free()

		createCommit(baseRepoPath, "some-file", []byte("credential"), "Initial commit")

		head, err = baseRepo.Head()
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("revok-scanner")
		gitClient = gitclient.New("private-key-path", "public-key-path")
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		repositoryRepository.FindReturns(db.Repository{
			Model: db.Model{
				ID: 42,
			},
			Path:  baseRepoPath,
			Owner: "some-owner",
			Name:  "some-repository",
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

		scanner = revok.NewScanner(gitClient, repositoryRepository, scanRepository, sniffer, emitter)
	})

	AfterEach(func() {
		head.Free()
		os.RemoveAll(baseRepoPath)
		os.RemoveAll(repoToScanPath)
	})

	It("sniffs", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", head.Target().String())
		Expect(err).NotTo(HaveOccurred())
		Eventually(sniffer.SniffCallCount).Should(Equal(1))
	})

	It("records credentials found in the repository", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", head.Target().String())
		Expect(err).NotTo(HaveOccurred())
		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
	})

	It("tries to store information in the database about found credentials", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", head.Target().String())
		Expect(err).NotTo(HaveOccurred())
		Eventually(scanRepository.StartCallCount).Should(Equal(1))
		_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
		Expect(scanType).To(Equal("diff-scan"))
		Expect(repository.ID).To(BeNumerically("==", 42))
		Expect(fetch).To(BeNil())

		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(firstScan.FinishCallCount).Should(Equal(1))
	})

	Context("when the repository has multiple commits", func() {
		BeforeEach(func() {
			createCommit(baseRepoPath, "some-other-file", []byte("credential"), "second commit")
		})

		It("scans all commits in the repository", func() {
			err := scanner.Scan(logger, "some-owner", "some-repository", head.Target().String())
			Expect(err).NotTo(HaveOccurred())
			Eventually(sniffer.SniffCallCount).Should(Equal(2))
			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
			Eventually(secondScan.RecordCredentialCallCount).Should(Equal(1))
		})
	})

	Context("when finding the repository fails", func() {
		BeforeEach(func() {
			repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
		})

		It("does not try to scan", func() {
			err := scanner.Scan(logger, "some-owner", "some-repository", head.Target().String())
			Expect(err).To(HaveOccurred())
			Consistently(scanRepository.StartCallCount).Should(BeZero())
		})
	})
})

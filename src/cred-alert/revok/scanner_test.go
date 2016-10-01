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
		successMetric  *metricsfakes.FakeCounter
		failedMetric   *metricsfakes.FakeCounter
		baseRepoPath   string
		repoToScanPath string
		baseRepo       *git.Repository
		result         createCommitResult
	)

	BeforeEach(func() {
		var err error
		baseRepoPath, err = ioutil.TempDir("", "revok-test-base-repo")
		Expect(err).NotTo(HaveOccurred())

		baseRepo, err = git.InitRepository(baseRepoPath, false)
		Expect(err).NotTo(HaveOccurred())

		result = createCommit("refs/heads/master", baseRepoPath, "some-file", []byte("credential"), "Initial commit")

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
		scanRepository.StartStub = func(lager.Logger, string, *db.Repository, *db.Fetch) db.ActiveScan {
			return firstScan
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
		baseRepo.Free()
		os.RemoveAll(baseRepoPath)
		os.RemoveAll(repoToScanPath)
	})

	It("sniffs", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(sniffer.SniffCallCount).Should(Equal(1))
	})

	It("records credentials found in the repository", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		credential := firstScan.RecordCredentialArgsForCall(0)
		Expect(credential.Owner).To(Equal("some-owner"))
		Expect(credential.Repository).To(Equal("some-repository"))
		Expect(credential.SHA).To(Equal(result.To.String()))
		Expect(credential.Path).To(Equal("some-file"))
	})

	It("tries to store information in the database about found credentials", func() {
		err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(scanRepository.StartCallCount).Should(Equal(1))
		_, scanType, repository, fetch := scanRepository.StartArgsForCall(0)
		Expect(scanType).To(Equal("repo-scan"))
		Expect(repository.ID).To(BeNumerically("==", 42))
		Expect(fetch).To(BeNil())

		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(firstScan.FinishCallCount).Should(Equal(1))
	})

	Context("when the repository has multiple commits", func() {
		var stopSHA string

		BeforeEach(func() {
			stopSHA = result.To.String()

			createCommit("refs/heads/master", baseRepoPath, "some-other-file", []byte("credential"), "second commit")
			result = createCommit("refs/heads/master", baseRepoPath, "yet-another-file", []byte("credential"), "third commit")
		})

		It("scans from the given SHA to the beginning of the repository", func() {
			err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), "")
			Expect(err).NotTo(HaveOccurred())
			Eventually(sniffer.SniffCallCount).Should(Equal(3))
			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(3))
			Eventually(firstScan.FinishCallCount).Should(Equal(1))
		})

		It("doesn't scan past the SHA to stop at if provided", func() {
			err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), stopSHA)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sniffer.SniffCallCount).Should(Equal(2))
			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(2))
		})
	})

	Context("when finding the repository fails", func() {
		BeforeEach(func() {
			repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
		})

		It("does not try to scan", func() {
			err := scanner.Scan(logger, "some-owner", "some-repository", result.To.String(), "")
			Expect(err).To(HaveOccurred())
			Consistently(scanRepository.StartCallCount).Should(BeZero())
		})
	})
})

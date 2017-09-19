package revok_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"gopkg.in/libgit2/git2go.v24"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/revok"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("Scanner", func() {
	var (
		logger               *lagertest.TestLogger
		gitClient            revok.GitGetParentsDiffClient
		sniffer              *snifffakes.FakeSniffer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		scanRepository       *dbfakes.FakeScanRepository
		credentialRepository *dbfakes.FakeCredentialRepository
		scanner              *revok.Scanner

		firstScan      *dbfakes.FakeActiveScan
		baseRepoPath   string
		repoToScanPath string
		baseRepo       *git.Repository
		result         createCommitResult
		scannedOids    map[string]struct{}
	)

	BeforeEach(func() {
		var err error
		baseRepoPath, err = ioutil.TempDir("", "revok-test-base-repo")
		Expect(err).NotTo(HaveOccurred())

		baseRepo, err = git.InitRepository(baseRepoPath, false)
		Expect(err).NotTo(HaveOccurred())

		result = createCommit("refs/heads/master", baseRepoPath, "some-file", []byte("credential"), "Initial commit", nil)

		logger = lagertest.NewTestLogger("revok-scanner")

		gitPath, err := exec.LookPath("git")
		Expect(err).NotTo(HaveOccurred())

		gitClient = gitclient.New("private-key-path", "public-key-path", gitPath)
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		repositoryRepository.MustFindReturns(db.Repository{
			Model: db.Model{
				ID: 42,
			},
			Path:    baseRepoPath,
			Owner:   "some-owner",
			Name:    "some-repository",
			Private: true,
		}, nil)

		scanRepository = &dbfakes.FakeScanRepository{}
		firstScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartStub = func(lager.Logger, string, string, string, string, *db.Repository) db.ActiveScan {
			return firstScan
		}

		credentialRepository = &dbfakes.FakeCredentialRepository{}

		scannedOids = map[string]struct{}{}
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

		scanner = revok.NewScanner(
			gitClient,
			repositoryRepository,
			scanRepository,
			credentialRepository,
			sniffer,
		)
	})

	AfterEach(func() {
		baseRepo.Free()
		os.RemoveAll(baseRepoPath)
		os.RemoveAll(repoToScanPath)
	})

	It("sniffs", func() {
		_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "branch", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(sniffer.SniffCallCount).Should(Equal(1))
	})

	It("records credentials found in the repository", func() {
		_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "branch", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		credential := firstScan.RecordCredentialArgsForCall(0)
		Expect(credential.Owner).To(Equal("some-owner"))
		Expect(credential.Repository).To(Equal("some-repository"))
		Expect(credential.SHA).To(Equal(result.To.String()))
		Expect(credential.Path).To(Equal("some-file"))
		Expect(credential.LineNumber).To(Equal(1))
		Expect(credential.MatchStart).To(Equal(1))
		Expect(credential.MatchEnd).To(Equal(2))
		Expect(credential.Private).To(BeTrue())
	})

	It("tries to store information in the database about found credentials", func() {
		_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Eventually(scanRepository.StartCallCount).Should(Equal(1))
		_, scanType, branch, startSHA, stopSHA, repository := scanRepository.StartArgsForCall(0)
		Expect(scanType).To(Equal("repo-scan"))
		Expect(branch).To(Equal("some-branch"))
		Expect(startSHA).To(Equal(result.To.String()))
		Expect(stopSHA).To(Equal(""))
		Expect(repository.ID).To(BeNumerically("==", 42))

		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(firstScan.FinishCallCount).Should(Equal(1))
	})

	It("returns credentials", func() {
		credentials, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())

		Expect(credentials).To(HaveLen(1))
		Expect(credentials[0].Owner).To(Equal("some-owner"))
		Expect(credentials[0].Repository).To(Equal("some-repository"))
		Expect(credentials[0].SHA).To(Equal(result.To.String()))
		Expect(credentials[0].Path).To(Equal("some-file"))
		Expect(credentials[0].LineNumber).To(Equal(1))
		Expect(credentials[0].Private).To(BeTrue())
	})

	Context("when there are no credentials found", func() {
		BeforeEach(func() {
			result = createCommit("refs/heads/topicA", baseRepoPath, "some-file", []byte("some-text"), "some-text commit", nil)
		})

		It("does not return any credentials", func() {
			credentials, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "refs/heads/topicA", result.To.String(), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(credentials).To(BeEmpty())
		})
	})

	Context("when the repository has multiple commits", func() {
		var stopSHA string

		BeforeEach(func() {
			stopSHA = result.To.String()

			createCommit("refs/heads/master", baseRepoPath, "some-other-file", []byte("credential"), "second commit", nil)
			result = createCommit("refs/heads/master", baseRepoPath, "yet-another-file", []byte("credential"), "third commit", nil)
		})

		It("scans from the given SHA to the beginning of the repository", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), "")
			Expect(err).NotTo(HaveOccurred())
			Eventually(sniffer.SniffCallCount).Should(Equal(3))
			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(3))
			Eventually(firstScan.FinishCallCount).Should(Equal(1))
		})

		It("doesn't scan past the SHA to stop at if provided", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), stopSHA)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sniffer.SniffCallCount).Should(Equal(2))
			Eventually(firstScan.RecordCredentialCallCount).Should(Equal(2))
		})

		It("starts the scan with the start and stop SHA", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), stopSHA)
			Expect(err).NotTo(HaveOccurred())

			_, scanType, branch, actualStartSHA, actualStopSHA, repository := scanRepository.StartArgsForCall(0)
			Expect(scanType).To(Equal("repo-scan"))
			Expect(branch).To(Equal("some-branch"))
			Expect(actualStartSHA).To(Equal(result.To.String()))
			Expect(actualStopSHA).To(Equal(stopSHA))
			Expect(repository.ID).To(BeNumerically("==", 42))
		})
	})

	Context("when the repository has a merge commit", func() {
		var mergeCommitResult createCommitResult

		BeforeEach(func() {
			By("creating three total commits on master")
			secondMasterCommitResult := createCommit("refs/heads/master", baseRepoPath, "second-commit-file", []byte("credential"), "second commit", nil)
			thirdMasterCommitResult := createCommit("refs/heads/master", baseRepoPath, "third-commit-file", []byte("credential"), "third commit", nil)

			By("creating a branch from master")
			firstTopicACommitResult := createCommit("refs/heads/topicA", baseRepoPath, "topic-a-file", []byte("credential"), "first topicA commit", secondMasterCommitResult.To)

			By("creating a merge commit between master and the branch")
			mergeCommitResult = createMerge(thirdMasterCommitResult.To, firstTopicACommitResult.To, baseRepoPath)
		})

		It("scans all parents", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", mergeCommitResult.To.String(), "")
			Expect(err).NotTo(HaveOccurred())

			actualCredPaths := []string{}
			for i := 0; i < firstScan.RecordCredentialCallCount(); i++ {
				credential := firstScan.RecordCredentialArgsForCall(i)
				actualCredPaths = append(actualCredPaths, credential.Path)
			}

			Expect(actualCredPaths).To(ConsistOf(
				"some-file",
				"second-commit-file",
				"third-commit-file",
				"topic-a-file",
			))
		})
	})

	Context("when the repository has been scanned in the past with the same scanner version and now has a new merge commit", func() {
		var mergeCommitResult createCommitResult
		var scan *dbfakes.FakeActiveScan

		BeforeEach(func() {
			secondMasterCommitResult := createCommit("refs/heads/master", baseRepoPath, "second-commit-file", []byte("credential"), "second commit", nil)
			thirdMasterCommitResult := createCommit("refs/heads/master", baseRepoPath, "third-commit-file", []byte("credential"), "third commit", nil)

			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", thirdMasterCommitResult.To.String(), "")
			Expect(err).NotTo(HaveOccurred())

			scan = &dbfakes.FakeActiveScan{}
			scanRepository.StartStub = func(lager.Logger, string, string, string, string, *db.Repository) db.ActiveScan {
				return scan
			}

			credentialRepository.UniqueSHAsForRepoAndRulesVersionReturns([]string{
				result.To.String(),
				secondMasterCommitResult.To.String(),
				thirdMasterCommitResult.To.String(),
			}, nil)

			firstTopicACommitResult := createCommit("refs/heads/topicA", baseRepoPath, "topic-a-file", []byte("credential"), "first topicA commit", secondMasterCommitResult.To)

			mergeCommitResult = createMerge(thirdMasterCommitResult.To, firstTopicACommitResult.To, baseRepoPath)
		})

		It("scans only the never-before seen commits", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", mergeCommitResult.To.String(), "")
			Expect(err).NotTo(HaveOccurred())

			actualCredPaths := []string{}
			for i := 0; i < scan.RecordCredentialCallCount(); i++ {
				credential := scan.RecordCredentialArgsForCall(i)
				actualCredPaths = append(actualCredPaths, credential.Path)
			}

			Expect(actualCredPaths).To(ConsistOf("topic-a-file"))
		})
	})

	It("does nothing when provided with a branch that's already been scanned", func() {
		m := map[string]struct{}{
			result.To.String(): struct{}{},
		}
		_, err := scanner.Scan(logger, "some-owner", "some-repository", m, "some-branch", result.To.String(), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(firstScan.RecordCredentialCallCount()).To(BeZero())
	})

	Context("when finding the repository fails", func() {
		BeforeEach(func() {
			repositoryRepository.MustFindReturns(db.Repository{}, errors.New("an-error"))
		})

		It("does not try to scan", func() {
			_, err := scanner.Scan(logger, "some-owner", "some-repository", scannedOids, "some-branch", result.To.String(), "")
			Expect(err).To(HaveOccurred())
			Consistently(scanRepository.StartCallCount).Should(BeZero())
		})
	})
})

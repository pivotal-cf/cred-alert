package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
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

var _ = Describe("Rescanner", func() {
	var (
		logger               *lagertest.TestLogger
		sniffer              *snifffakes.FakeSniffer
		repositoryRepository *dbfakes.FakeRepositoryRepository
		scanRepository       *dbfakes.FakeScanRepository
		emitter              *metricsfakes.FakeEmitter

		firstScan     *dbfakes.FakeActiveScan
		secondScan    *dbfakes.FakeActiveScan
		successMetric *metricsfakes.FakeCounter
		failedMetric  *metricsfakes.FakeCounter

		tempdir string
		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
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

		scanRepository = &dbfakes.FakeScanRepository{}
		firstScan = &dbfakes.FakeActiveScan{}
		secondScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartStub = func(lager.Logger, string, string, string, *db.Repository, *db.Fetch) db.ActiveScan {
			if scanRepository.StartCallCount() == 1 {
				return firstScan
			}
			return secondScan
		}

		var err error
		tempdir, err = ioutil.TempDir("", "dirscan-updater-test")
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(filepath.Join(tempdir, "some-owner", "some-repo"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(tempdir, "some-owner", "some-repo", "some-file"), []byte("credential"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(filepath.Join(tempdir, "some-other-owner", "some-other-repo"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(tempdir, "some-other-owner", "some-other-repo", "some-other-file"), []byte("credential"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		repositoryRepository.NotScannedWithVersionReturns([]db.Repository{
			{
				Model: db.Model{
					ID: 42,
				},
				Name:  "some-repo",
				Owner: "some-owner",
				Path:  filepath.Join(tempdir, "some-owner", "some-repo"),
			},
			{
				Model: db.Model{
					ID: 44,
				},
				Name:  "some-other-repo",
				Owner: "some-other-owner",
				Path:  filepath.Join(tempdir, "some-other-owner", "some-other-repo"),
			},
		}, nil)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
		os.RemoveAll(tempdir)
	})

	JustBeforeEach(func() {
		runner = revok.NewRescanner(
			logger,
			sniffer,
			repositoryRepository,
			scanRepository,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	It("gets the repositories from the RepositoryRepository", func() {
		Eventually(repositoryRepository.NotScannedWithVersionCallCount).Should(Equal(1))
		rulesVersion := repositoryRepository.NotScannedWithVersionArgsForCall(0)
		Expect(rulesVersion).To(Equal(sniff.RulesVersion))
	})

	It("does a dirscan on each repository", func() {
		Eventually(scanRepository.StartCallCount).Should(Equal(2))

		_, scanType, _, _, repository, fetch := scanRepository.StartArgsForCall(0)
		Expect(scanType).To(Equal("dir-scan"))
		Expect(repository.ID).To(BeNumerically("==", 42))
		Expect(fetch).To(BeNil())

		Eventually(firstScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(firstScan.FinishCallCount).Should(Equal(1))

		_, scanType, _, _, repository, fetch = scanRepository.StartArgsForCall(1)
		Expect(scanType).To(Equal("dir-scan"))
		Expect(repository.ID).To(BeNumerically("==", 44))
		Expect(fetch).To(BeNil())

		Eventually(secondScan.RecordCredentialCallCount).Should(Equal(1))
		Eventually(secondScan.FinishCallCount).Should(Equal(1))
	})

	Context("when getting repositories fails", func() {
		BeforeEach(func() {
			repositoryRepository.NotScannedWithVersionReturns(nil, errors.New("an-error"))
		})

		It("does not try to dirscan anything", func() {
			Consistently(scanRepository.StartCallCount).Should(Equal(0))
		})
	})
})

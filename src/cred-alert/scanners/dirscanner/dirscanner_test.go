package dirscanner_test

import (
	"archive/tar"
	"cred-alert/scanners"
	"cred-alert/scanners/dirscanner"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DirScanner", func() {
	var (
		scanner          *dirscanner.DirScanner
		sniffer          *snifffakes.FakeSniffer
		logger           *lagertest.TestLogger
		credentialCount  int
		handlerCallCount int
		handler          sniff.ViolationHandlerFunc
		inflateDir       string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("dirscanner")

		handlerCallCount = 0
		handler = func(l lager.Logger, violation scanners.Violation) error {
			handlerCallCount++
			return nil
		}

		credentialCount = 0
		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			for s.Scan(l) {
				line := s.Line(l)
				if strings.Contains(string(line.Content), "credential") {
					h(l, scanners.Violation{})
					credentialCount++
				}
			}
			return nil
		}

		inflateDir = ""
	})

	JustBeforeEach(func() {
		scanner = dirscanner.New(sniffer, handler, inflateDir)
	})

	Describe("Scan", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "testdir")
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("credential"), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("credential"), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tmpDir)
		})

		It("scans a directory", func() {
			err := scanner.Scan(logger, tmpDir)
			Expect(err).NotTo(HaveOccurred())

			Expect(sniffer.SniffCallCount()).To(Equal(2))
			Expect(credentialCount).To(Equal(2))
			Expect(handlerCallCount).To(Equal(2))
		})

		Context("when the directory contains an archive", func() {
			var violationDir string

			BeforeEach(func() {
				tarfilePath := filepath.Join(tmpDir, "archive.tar")
				tarfile, err := os.Create(tarfilePath)
				Expect(err).NotTo(HaveOccurred())
				defer tarfile.Close()

				tw := tar.NewWriter(tarfile)
				err = tw.WriteHeader(&tar.Header{
					Name: "file3.txt",
					Mode: 0600,
					Size: int64(len("credential")),
				})
				Expect(err).NotTo(HaveOccurred())
				defer tw.Close()

				_, err = tw.Write([]byte("credential"))
				Expect(err).NotTo(HaveOccurred())

				inflateDir, err = ioutil.TempDir("", "dirscanner-test-inflate-dir")
				Expect(err).NotTo(HaveOccurred())

				violationDir, err = ioutil.TempDir("", "dirscanner-test-violation-dir")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(inflateDir)
				os.RemoveAll(violationDir)
			})

			It("scans the archive", func() {
				err := scanner.Scan(logger, tmpDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(sniffer.SniffCallCount()).To(Equal(3))
				Expect(credentialCount).To(Equal(3))
			})
		})
	})
})

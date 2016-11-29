package dirscanner_test

import (
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
	var scanner *dirscanner.DirScanner
	var sniffer *snifffakes.FakeSniffer
	var logger *lagertest.TestLogger
	var credentialCount int

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("dirscanner")
		credentialCount = 0
		handler := func(l lager.Logger, violation scanners.Violation) error {
			return nil
		}
		sniffer = &snifffakes.FakeSniffer{}
		sniffer.SniffStub = func(l lager.Logger, s sniff.Scanner, h sniff.ViolationHandlerFunc) error {
			for s.Scan(l) {
				line := s.Line(l)
				if strings.Contains(string(line.Content), "credential") {
					credentialCount++
				}
			}
			return nil
		}

		scanner = dirscanner.New(handler, sniffer)
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
	})
})

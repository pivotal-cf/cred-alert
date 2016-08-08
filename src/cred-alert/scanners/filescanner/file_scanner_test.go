package filescanner_test

import (
	"io/ioutil"
	"os"

	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("File", func() {
	var (
		fileScanner sniff.Scanner

		fileHandle *os.File
		logger     lager.Logger
	)

	fileContent := `line1
line2
line3`

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("file-scanner")

		var err error
		fileHandle, err = ioutil.TempFile("", "file-scanner-test-temp")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(fileHandle.Name(), []byte(fileContent), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := fileHandle.Close()
		Expect(err).NotTo(HaveOccurred())

		os.RemoveAll(fileHandle.Name())
	})

	JustBeforeEach(func() {
		fileScanner = filescanner.New(fileHandle, fileHandle.Name())
	})

	It("returns true when the scan results in a line", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeFalse())
	})

	It("returns the current line", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		line := fileScanner.Line(logger)

		Expect(line.Path).To(Equal(fileHandle.Name()))
		Expect(line.Content).To(ContainSubstring("line1"))
		Expect(line.LineNumber).To(Equal(1))
	})

	It("keeps track of line numbers", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		line := fileScanner.Line(logger)
		Expect(line.LineNumber).To(Equal(3))
	})
})

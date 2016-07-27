package filescanner_test

import (
	"io/ioutil"
	"os"

	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
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

	It("scans lines from a file", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeFalse())
	})

	It("returns the current line", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		line := fileScanner.Line()

		Expect(line.Path).To(Equal(fileHandle.Name()))
		Expect(line.Content).To(Equal("line1"))
		Expect(line.LineNumber).To(Equal(1))
	})

	It("keeps track of line numbers", func() {
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		Expect(fileScanner.Scan(logger)).To(BeTrue())
		line := fileScanner.Line()
		Expect(line.LineNumber).To(Equal(3))
	})
})

package file_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"cred-alert/scanners/file"
	"cred-alert/sniff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("File", func() {
	var (
		myFile       *os.File
		tempFilePath string
		fileContent  string
		fileScanner  sniff.Scanner
		logger       lager.Logger
	)

	BeforeEach(func() {
		fileContent = `line1
line2
line3`

		tempFilePath = fmt.Sprintf("%s/file-scanner-test-temp", os.TempDir())
		if err := ioutil.WriteFile(tempFilePath, []byte(fileContent), 0644); err != nil {
			fmt.Println(err)
		}
		var err error
		myFile, err = os.Open(tempFilePath)
		if err != nil {
			fmt.Println(err)
		}

		logger = lagertest.NewTestLogger("file-scanner")
	})

	AfterEach(func() {
		myFile.Close()
		os.Remove(myFile.Name())
	})

	JustBeforeEach(func() {
		fileScanner = file.NewFileScanner(myFile)
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

		Expect(line.Path).To(Equal(tempFilePath))
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

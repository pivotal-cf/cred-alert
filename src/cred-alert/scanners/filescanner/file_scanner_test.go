package filescanner_test

import (
	"errors"
	"io"
	"io/ioutil"
	"os"

	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("File", func() {
	var (
		fileScanner sniff.Scanner

		fileHandle *os.File
		readCloser io.ReadCloser
		fileName   string
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
		fileName = fileHandle.Name()

		err = ioutil.WriteFile(fileHandle.Name(), []byte(fileContent), 0644)
		Expect(err).NotTo(HaveOccurred())
		readCloser = fileHandle
	})

	AfterEach(func() {
		err := fileHandle.Close()
		Expect(err).NotTo(HaveOccurred())

		if fileName != "" {
			os.RemoveAll(fileName)
		}
	})

	JustBeforeEach(func() {
		fileScanner = filescanner.New(readCloser, fileName)
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

	Context("when the file reader errors", func() {
		BeforeEach(func() {
			readCloser = &ErrReader{Err: errors.New("my awesome error")}
			fileName = ""
		})

		It("returns any error encountered while scanning", func() {
			Expect(fileScanner.Scan(logger)).To(BeFalse())
			Expect(fileScanner.Err()).To(HaveOccurred())
		})
	})
})

type ErrReader struct {
	Err error
}

func (r *ErrReader) Close() error {
	return nil
}

func (r *ErrReader) Read(b []byte) (int, error) {
	return 0, r.Err
}

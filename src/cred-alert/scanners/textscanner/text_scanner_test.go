package textscanner_test

import (
	"cred-alert/scanners/textscanner"
	"cred-alert/sniff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Text Scanner", func() {

	var (
		scanner sniff.Scanner
		logger  *lagertest.TestLogger
	)

	text := `line1
line2
line3`

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("text-scanner")
	})

	JustBeforeEach(func() {
		scanner = textscanner.New(text)
	})

	It("scans lines from a file", func() {
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Scan(logger)).To(BeFalse())
	})

	It("returns the current line", func() {
		Expect(scanner.Scan(logger)).To(BeTrue())
		line := scanner.Line(logger)

		Expect(line.Path).To(Equal("text"))
		Expect(line.Content).To(Equal("line1"))
		Expect(line.LineNumber).To(Equal(1))
	})

	It("keeps track of line numbers", func() {
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Line(logger).LineNumber).To(Equal(1))
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Line(logger).LineNumber).To(Equal(2))
		Expect(scanner.Scan(logger)).To(BeTrue())
		Expect(scanner.Line(logger).LineNumber).To(Equal(3))
	})
})

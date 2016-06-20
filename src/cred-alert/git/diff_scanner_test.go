package git_test

import (
	"cred-alert/git"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiffScanner", func() {
	shortFile := `+++ b/our/path/somefile.txt
@@ -4,7 +5,7 @@ some hint line
 first line of content
 second line of content`

	sneakyFile := `+++ b/our/path/somefile.txt
@@ -4,7 +5,7 @@ some hint line
 first line of content
 second line of content
+++sneaky line of content
 last line of content`

	It("scans lines from a diff", func() {
		diffScanner := git.NewDiffScanner(shortFile)
		Expect(diffScanner.Scan()).To(BeTrue())
		Expect(diffScanner.Scan()).To(BeTrue())
		Expect(diffScanner.Scan()).To(BeFalse())
	})

	It("returns the current line from a diff", func() {
		diffScanner := git.NewDiffScanner(shortFile)
		diffScanner.Scan()
		line := diffScanner.Line()

		Expect(line.Path).To(Equal("our/path/somefile.txt"))
		Expect(line.Content).To(Equal(`first line of content`))
		Expect(line.LineNumber).To(Equal(5))

		diffScanner.Scan()

		Expect(diffScanner.Line().Path).To(Equal("our/path/somefile.txt"))
		Expect(diffScanner.Line().Content).To(Equal("second line of content"))
		Expect(diffScanner.Line().LineNumber).To(Equal(6))
	})

	It("scans for a filename", func() {
		diffScanner := git.NewDiffScanner(shortFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().Path).To(Equal("our/path/somefile.txt"))
	})

	It("Is not fooled by lines that look like file headers", func() {
		diffScanner := git.NewDiffScanner(sneakyFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().Path).To(Equal("our/path/somefile.txt"))
		Expect(diffScanner.Line().Content).To(Equal("first line of content"))
		diffScanner.Scan()
		Expect(diffScanner.Line().Content).To(Equal("second line of content"))
		diffScanner.Scan()
		Expect(diffScanner.Line().Content).To(Equal("++sneaky line of content"))
	})

	It("scans for a hunk", func() {
		diffScanner := git.NewDiffScanner(shortFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(5))
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(6))
	})

	It("scans multiple hunks in one diff", func() {
		diffScanner := git.NewDiffScanner(sampleDiff)
		for i := 0; i < 8; i++ {
			diffScanner.Scan()
			fmt.Printf("%d: %s\n", diffScanner.Line().LineNumber, diffScanner.Line().Content)
		}

		Expect(diffScanner.Line().LineNumber).To(Equal(36))
		Expect(diffScanner.Line().Content).To(Equal("## Special Characters"))
	})

	It("keeps track of the filename in sections of a unified diff", func() {
	})

	It("keeps track of line numbers in sections of a unified diff", func() {
	})
})

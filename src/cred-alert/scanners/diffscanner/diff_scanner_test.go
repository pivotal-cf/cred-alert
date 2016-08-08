package diffscanner_test

import (
	"cred-alert/scanners/diffscanner"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
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

	singleLineRemovedFile := `diff --git a/stuff.txt b/stuff.txt
index f2e4113..fa5a232 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1 +1,2 @@
-stuff
+blah
+lol`

	singleLineAddedFile := `--git a/stuff.txt b/stuff.txt
index fa5a232..1e13fe8 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1,2 +1 @@
-blah
-lol
+rofl`

	singleLineReplacementFile := `diff --git a/stuff.txt b/stuff.txt
index 1e13fe8..06b14f8 100644
--- a/stuff.txt
+++ b/stuff.txt
@@ -1 +1 @@
-rofl
+afk`

	var logger *lagertest.TestLogger

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("diff-scanner-test")
	})

	It("scans lines from a diff", func() {
		diffScanner := diffscanner.NewDiffScanner(shortFile)
		Expect(diffScanner.Scan(logger)).To(BeTrue())
		Expect(diffScanner.Scan(logger)).To(BeTrue())
		Expect(diffScanner.Scan(logger)).To(BeFalse())
	})

	It("returns the current line from a diff", func() {
		diffScanner := diffscanner.NewDiffScanner(shortFile)

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).Path).To(Equal("our/path/somefile.txt"))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte(`first line of content`)))
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(5))

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).Path).To(Equal("our/path/somefile.txt"))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("second line of content")))
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(6))
	})

	It("scans for a filename", func() {
		diffScanner := diffscanner.NewDiffScanner(shortFile)
		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).Path).To(Equal("our/path/somefile.txt"))
	})

	It("Is not fooled by lines that look like file headers", func() {
		diffScanner := diffscanner.NewDiffScanner(sneakyFile)
		diffScanner.Scan(logger)

		Expect(diffScanner.Line(logger).Path).To(Equal("our/path/somefile.txt"))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("first line of content")))

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("second line of content")))

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("++sneaky line of content")))
	})

	It("scans for a hunk", func() {
		diffScanner := diffscanner.NewDiffScanner(shortFile)
		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(5))
		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(6))
	})

	It("scans multiple hunks in one diff", func() {
		diffScanner := diffscanner.NewDiffScanner(sampleDiff)

		for i := 0; i < 8; i++ {
			diffScanner.Scan(logger)
			fmt.Fprintf(
				GinkgoWriter,
				"%d: %s\n",
				diffScanner.Line(logger).LineNumber,
				diffScanner.Line(logger).Content,
			)
		}

		Expect(diffScanner.Line(logger).LineNumber).To(Equal(36))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("## Special Characters")))
	})

	It("scans single line hunks", func() {
		diffScanner := diffscanner.NewDiffScanner(singleLineRemovedFile)

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(1))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("blah")))

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(2))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("lol")))
		Expect(diffScanner.Scan(logger)).To(BeFalse())

		diffScanner = diffscanner.NewDiffScanner(singleLineAddedFile)

		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(1))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("rofl")))
		Expect(diffScanner.Scan(logger)).To(BeFalse())

		diffScanner = diffscanner.NewDiffScanner(singleLineReplacementFile)
		diffScanner.Scan(logger)
		Expect(diffScanner.Line(logger).LineNumber).To(Equal(1))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("afk")))
		Expect(diffScanner.Scan(logger)).To(BeFalse())
	})

	It("keeps track of the filename in sections of a unified diff", func() {
		diffScanner := diffscanner.NewDiffScanner(sampleDiff)
		for i := 0; i < 30; i++ {
			diffScanner.Scan(logger)
			fmt.Fprintf(
				GinkgoWriter,
				"%d: %s\n",
				diffScanner.Line(logger).LineNumber,
				diffScanner.Line(logger).Content,
			)
		}

		Expect(diffScanner.Line(logger).LineNumber).To(Equal(28))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte("private_key '$should_match'")))
		Expect(diffScanner.Line(logger).Path).To(Equal("spec/integration/git-secrets-pattern-tests2.txt"))
	})

	It("keeps track of line numbers in sections of a unified diff", func() {
		diffScanner := diffscanner.NewDiffScanner(sampleDiff)
		for i := 0; i < 5; i++ {
			diffScanner.Scan(logger)
			fmt.Fprintf(
				GinkgoWriter,
				"%d: %s\n",
				diffScanner.Line(logger).LineNumber,
				diffScanner.Line(logger).Content,
			)
		}

		Expect(diffScanner.Line(logger).LineNumber).To(Equal(32))
		Expect(diffScanner.Line(logger).Content).To(Equal([]byte(`hard_coded_salt: "should_match"`)))
		Expect(diffScanner.Line(logger).Path).To(Equal("spec/integration/git-secrets-pattern-tests.txt"))
	})
})

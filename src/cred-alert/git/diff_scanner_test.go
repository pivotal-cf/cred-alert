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
			fmt.Fprintf(GinkgoWriter, "%d: %s\n", diffScanner.Line().LineNumber, diffScanner.Line().Content)
		}

		Expect(diffScanner.Line().LineNumber).To(Equal(36))
		Expect(diffScanner.Line().Content).To(Equal("## Special Characters"))
	})

	It("scans single line hunks", func() {
		diffScanner := git.NewDiffScanner(singleLineRemovedFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(1))
		Expect(diffScanner.Line().Content).To(Equal("blah"))
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(2))
		Expect(diffScanner.Line().Content).To(Equal("lol"))
		Expect(diffScanner.Scan()).To(BeFalse())

		diffScanner = git.NewDiffScanner(singleLineAddedFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(1))
		Expect(diffScanner.Line().Content).To(Equal("rofl"))
		Expect(diffScanner.Scan()).To(BeFalse())

		diffScanner = git.NewDiffScanner(singleLineReplacementFile)
		diffScanner.Scan()
		Expect(diffScanner.Line().LineNumber).To(Equal(1))
		Expect(diffScanner.Line().Content).To(Equal("afk"))
		Expect(diffScanner.Scan()).To(BeFalse())
	})

	It("keeps track of the filename in sections of a unified diff", func() {
		diffScanner := git.NewDiffScanner(sampleDiff)
		for i := 0; i < 30; i++ {
			diffScanner.Scan()
			fmt.Fprintf(GinkgoWriter, "%d: %s\n", diffScanner.Line().LineNumber, diffScanner.Line().Content)
		}

		Expect(diffScanner.Line().LineNumber).To(Equal(28))
		Expect(diffScanner.Line().Content).To(Equal("private_key '$should_match'"))
		Expect(diffScanner.Line().Path).To(Equal("spec/integration/git-secrets-pattern-tests2.txt"))
	})

	It("keeps track of line numbers in sections of a unified diff", func() {
		diffScanner := git.NewDiffScanner(sampleDiff)
		for i := 0; i < 5; i++ {
			diffScanner.Scan()
			fmt.Fprintf(GinkgoWriter, "%d: %s\n", diffScanner.Line().LineNumber, diffScanner.Line().Content)
		}

		Expect(diffScanner.Line().LineNumber).To(Equal(32))
		Expect(diffScanner.Line().Content).To(Equal(`hard_coded_salt: "should_match"`))
		Expect(diffScanner.Line().Path).To(Equal("spec/integration/git-secrets-pattern-tests.txt"))

	})
})

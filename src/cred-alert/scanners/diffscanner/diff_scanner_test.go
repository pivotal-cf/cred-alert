package diffscanner_test

import (
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"strings"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiffScanner", func() {
	diffWithMultipleHunks := `diff --git a/some-file.txt b/some-file.txt
index 4d7fc47..1ba07b5 100644
--- a/some-file.txt
+++ b/some-file.txt
@@ -8,7 +8,7 @@ here is a file
 there are many like it
 but this one is mine
 here is a file
-there are many like it
+first changed line
 but this one is mine
 here is a file
 there are many like it
@@ -46,7 +46,7 @@ but this one is mine
 here is a file
 there are many like it
 but this one is mine
-here is a file
+second changed line
 there are many like it
 but this one is mine
 here is a file`

	diffWithMultipleHunksAndFiles := `diff --git a/some-file.txt b/some-file.txt
index 4d7fc47..1ba07b5 100644
--- a/some-file.txt
+++ b/some-file.txt
@@ -8,7 +8,7 @@ here is a file
 there are many like it
 but this one is mine
 here is a file
-there are many like it
+changed
 but this one is mine
 here is a file
 there are many like it
@@ -46,7 +46,7 @@ but this one is mine
 here is a file
 there are many like it
 but this one is mine
-here is a file
+changed
 there are many like it
 but this one is mine
 here is a file
diff --git a/some-other-file.txt b/some-other-file.txt
index 468bc36..4c112c7 100644
--- a/some-other-file.txt
+++ b/some-other-file.txt
@@ -27,6 +27,7 @@ here are file b contents
 here are file b contents
 here are file b contents
 here are file b contents
+changed
 here are file b contents
 here are file b contents
 here are file b contents
@@ -49,7 +50,6 @@ here are file b contents
 here are file b contents
 here are file b contents
 here are file b contents
-here are file b contents
-here are file b contents
+changed
 here are file b contents
 here are file b contents`

	var (
		logger  *lagertest.TestLogger
		scanner *diffscanner.DiffScanner
		diff    string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("diff-scanner-test")
		diff = diffWithMultipleHunks
	})

	JustBeforeEach(func() {
		scanner = diffscanner.NewDiffScanner(strings.NewReader(diff))
	})

	Describe("Scan", func() {
		Context("when the diff has added lines", func() {
			BeforeEach(func() {
				diff = diffWithMultipleHunks
			})

			It("returns true for each added line", func() {
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Scan(logger)).To(BeFalse())
			})
		})

		Context("when the diff has no added lines", func() {
			BeforeEach(func() {
				diff = `diff --git a/some-file.txt b/some-file.txt
index dbb2891..c2bce43 100644
--- a/some-file.txt
+++ b/some-file.txt
@@ -1,2 +1 @@
 first line of content
-second line of content`
			})

			It("returns false", func() {
				Expect(scanner.Scan(logger)).To(BeFalse())
			})
		})
	})

	Describe("Line", func() {
		It("returns an empty line", func() {
			line := scanner.Line(logger)
			Expect(*line).To(Equal(scanners.Line{
				Content:    nil,
				LineNumber: 0,
				Path:       "",
			}))
		})

		Context("when the diff has an added/changed line", func() {
			BeforeEach(func() {
				diff = diffWithMultipleHunks
			})

			It("returns a Line equivalent to the matched line after calling Scan()", func() {
				Expect(scanner.Scan(logger)).To(BeTrue())
				line := scanner.Line(logger)
				Expect(line.Path).To(Equal("some-file.txt"))
				Expect(line.Content).To(Equal([]byte("first changed line")))
				Expect(line.LineNumber).To(Equal(11))

				Expect(scanner.Scan(logger)).To(BeTrue())
				line = scanner.Line(logger)
				Expect(line.Path).To(Equal("some-file.txt"))
				Expect(line.Content).To(Equal([]byte("second changed line")))
				Expect(line.LineNumber).To(Equal(49))
			})

			Context("when an added/changed line looks like a file header", func() {
				BeforeEach(func() {
					diff = `diff --git a/some-file.txt b/some-file.txt
index dbb2891..5751378 100644
--- a/some-file.txt
+++ b/some-file.txt
@@ -1,2 +1,3 @@
 first line of content
 second line of content
+++new line`
				})

				It("returns a Line equivalent to the matched line after calling Scan()", func() {
					Expect(scanner.Scan(logger)).To(BeTrue())
					line := scanner.Line(logger)
					Expect(line.Path).To(Equal("some-file.txt"))
					Expect(line.Content).To(Equal([]byte("++new line")))
					Expect(line.LineNumber).To(Equal(3))
				})
			})
		})

		Context("when the diff has multiple files", func() {
			BeforeEach(func() {
				diff = diffWithMultipleHunksAndFiles
			})

			It("keeps track of the filename in sections of a unified diff", func() {
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Line(logger).Path).To(Equal("some-file.txt"))
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Line(logger).Path).To(Equal("some-file.txt"))
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Line(logger).Path).To(Equal("some-other-file.txt"))
				Expect(scanner.Scan(logger)).To(BeTrue())
				Expect(scanner.Line(logger).Path).To(Equal("some-other-file.txt"))
			})
		})
	})

})

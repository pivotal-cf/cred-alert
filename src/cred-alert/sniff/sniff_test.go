package sniff_test

import (
	"cred-alert/scanners/git"
	"cred-alert/sniff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Scan", func() {

	shortDiff := `diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests.txt
+++ b/spec/integration/git-secrets-pattern-tests.txt
@@ -28,3 +28,3 @@ private_key = "should_match" # TODO: comments shouldn't have an effect
 private_key '$should_match'
 # Should Not Match
+
@@ -67,6 +75,5 @@ private_key: "should not match"
 private_key: "too-short" # should_not_match
 private_key: "fake_should_not_match"
+private_key: "should_match"
+private_key: "FaKe_should_not_match"
+private_key: "ExAmPlE_should_not_match"
`

	It("scans a diff and return Lines", func() {
		logger := lagertest.NewTestLogger("scanner")
		scanner := git.NewDiffScanner(shortDiff)
		called := 0
		handleViolation := func(sniff.Line) {
			called++
		}
		sniff.Sniff(logger, scanner, handleViolation)
		Expect(called).To(Equal(2))
	})
})

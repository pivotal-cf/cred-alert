package patterns_test

import (
	"cred-alert/sniff/matchers"
	"cred-alert/sniff/patterns"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Default patterns", func() {
	var lines []string
	var matcher patterns.Matcher

	BeforeEach(func() {
		lines = strings.Split(sample_strings, "\n")
		matcher = patterns.DefaultMatcher()
	})

	It("matches all positive examples", func() {
		for _, line := range lines {
			if exclusionMatcher.Match(line) {
				continue
			}

			shouldMatch := strings.Contains(line, "should_match")
			found := matcher.Match(line)

			Expect(found).To(Equal(shouldMatch), line)
		}
	})
})

const bashStringInterpolationPattern = `["]\$`
const fakePattern = `(?i)fake`
const examplePattern = `(?i)example`

var exclusionMatcher = matchers.Multi(
	matchers.KnownFormat(bashStringInterpolationPattern),
	matchers.KnownFormat(fakePattern),
	matchers.KnownFormat(examplePattern),
)

package ingestor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/ingestor"
)

var _ = Describe("Whitelist", func() {
	It("determines whether or not a repository should be ignored", func() {
		whitelist := ingestor.BuildWhitelist(
			".*-ci",
			"deployments-.*",
		)

		Expect(whitelist.IsIgnored("not-matching")).To(BeFalse())
		Expect(whitelist.IsIgnored("team-ci")).To(BeTrue())
		Expect(whitelist.IsIgnored("deployments-matching")).To(BeTrue())
	})

	It("automatically anchors the regexs", func() {
		whitelist := ingestor.BuildWhitelist(
			".*-ci",
			"deployments-.*",
		)

		Expect(whitelist.IsIgnored("other-deployments-thing")).To(BeFalse())
		Expect(whitelist.IsIgnored("other-ci-thing")).To(BeFalse())
	})

	It("doesn't break pre-anchored regexps", func() {
		whitelist := ingestor.BuildWhitelist(
			"^.*-ci$",
			"^deployments-.*$",
		)

		Expect(whitelist.IsIgnored("other-deployments-thing")).To(BeFalse())
		Expect(whitelist.IsIgnored("other-ci-thing")).To(BeFalse())
		Expect(whitelist.IsIgnored("not-matching")).To(BeFalse())
		Expect(whitelist.IsIgnored("team-ci")).To(BeTrue())
		Expect(whitelist.IsIgnored("deployments-matching")).To(BeTrue())
	})
})

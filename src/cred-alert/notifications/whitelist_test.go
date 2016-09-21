package notifications_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/notifications"
)

var _ = Describe("Whitelist", func() {
	var private bool

	Context("when the repository is public", func() {
		BeforeEach(func() {
			private = false
		})

		It("should never skip a notification", func() {
			whitelist := notifications.BuildWhitelist(
				".*-ci",
				"deployments-.*",
			)

			Expect(whitelist.ShouldSkipNotification(private, "not-matching")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "team-ci")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "deployments-matching")).To(BeFalse())
		})
	})

	Context("when the repository is private", func() {
		BeforeEach(func() {
			private = true
		})

		It("determines whether or not a repository should be ignored", func() {
			whitelist := notifications.BuildWhitelist(
				".*-ci",
				"deployments-.*",
			)

			Expect(whitelist.ShouldSkipNotification(private, "not-matching")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "team-ci")).To(BeTrue())
			Expect(whitelist.ShouldSkipNotification(private, "deployments-matching")).To(BeTrue())
		})

		It("automatically anchors the regexs", func() {
			whitelist := notifications.BuildWhitelist(
				".*-ci",
				"deployments-.*",
			)

			Expect(whitelist.ShouldSkipNotification(private, "other-deployments-thing")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "other-ci-thing")).To(BeFalse())
		})

		It("doesn't break pre-anchored regexps", func() {
			whitelist := notifications.BuildWhitelist(
				"^.*-ci$",
				"^deployments-.*$",
			)

			Expect(whitelist.ShouldSkipNotification(private, "other-deployments-thing")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "other-ci-thing")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "not-matching")).To(BeFalse())
			Expect(whitelist.ShouldSkipNotification(private, "team-ci")).To(BeTrue())
			Expect(whitelist.ShouldSkipNotification(private, "deployments-matching")).To(BeTrue())
		})
	})
})

package notifications_test

import (
	"cred-alert/notifications"
	"cred-alert/scanners"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Notifications", func() {
	var slackNotifier notifications.Notifier
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("slack-notifier")
	})

	Context("nil webhookUrl", func() {
		BeforeEach(func() {
			slackNotifier = notifications.NewSlackNotifier("")
		})

		It("Returns a nullNotifier", func() {
			Expect(slackNotifier).NotTo(BeNil())
		})

		It("handles sending notifications", func() {
			err := slackNotifier.SendNotification(logger, "owner/repo", "123abc", scanners.Line{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Slack notifications", func() {
		var server *ghttp.Server

		BeforeEach(func() {
			server = ghttp.NewServer()
			slackNotifier = notifications.NewSlackNotifier(server.URL())
		})

		It("POSTs a message to the fake slack webhook", func() {
			expectedJSON := `{
				"attachments": [
					{
						"title": "Credential detected in owner/repo!",
						"text": "<https://github.com/owner/repo/blob/abc123/path/to/file.txt#L123|path/to/file.txt:123>",
						"color": "danger",
						"fallback": "https://github.com/owner/repo/blob/abc123/path/to/file.txt#L123"
					}
				]
			}
			`

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyJSON(expectedJSON),
				),
			)

			slackNotifier.SendNotification(logger, "owner/repo", "abc123", scanners.Line{Path: "path/to/file.txt", LineNumber: 123})

			Expect(server.ReceivedRequests()).Should(HaveLen(1))
		})
	})
})

package notifications_test

import (
	"cred-alert/notifications"

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
			slackNotifier = notifications.NewSlackNotifier(logger, "")
		})

		It("Returns a nullNotifier", func() {
			Expect(slackNotifier).NotTo(BeNil())
		})

		It("handles sending notifications", func() {
			err := slackNotifier.SendNotification("something happened")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Slack notifications", func() {
		var server *ghttp.Server

		BeforeEach(func() {
			server = ghttp.NewServer()
			slackNotifier = notifications.NewSlackNotifier(logger, server.URL())
		})

		It("POSTs a message to the fake slack webhook", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyBody([]byte(`{"text":"some message"}`)),
				),
			)
			slackNotifier.SendNotification("some message")
			Expect(server.ReceivedRequests()).Should(HaveLen(1))
		})
	})
})

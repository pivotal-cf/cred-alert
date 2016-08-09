package notifications_test

import (
	"cred-alert/notifications"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Notifications", func() {
	var (
		slackNotifier notifications.Notifier
		clock         *fakeclock.FakeClock
		logger        *lagertest.TestLogger
		private       bool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("slack-notifier")
		clock = fakeclock.NewFakeClock(time.Now())
		private = true
	})

	Context("nil webhookUrl", func() {
		BeforeEach(func() {
			slackNotifier = notifications.NewSlackNotifier("", clock)
		})

		It("Returns a nullNotifier", func() {
			Expect(slackNotifier).NotTo(BeNil())
		})

		It("handles sending notifications", func() {
			err := slackNotifier.SendNotification(logger, notifications.Notification{
				Owner:      "owner",
				Repository: "repo",
				Private:    private,
				SHA:        "123abc",
				Path:       "a/path/to/a/file",
				LineNumber: 42,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Slack notifications", func() {
		var server *ghttp.Server

		BeforeEach(func() {
			server = ghttp.NewServer()
			slackNotifier = notifications.NewSlackNotifier(server.URL(), clock)
		})

		It("POSTs a message to the fake slack webhook", func() {
			expectedJSON := notificationJSON("warning")

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyJSON(expectedJSON),
				),
			)

			err := slackNotifier.SendNotification(logger, notifications.Notification{
				Owner:      "owner",
				Repository: "repo",
				Private:    private,
				SHA:        "abc123",
				Path:       "path/to/file.txt",
				LineNumber: 123,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(server.ReceivedRequests()).Should(HaveLen(1))
		})

		Context("when Slack responds with an 429 Too Many Requests", func() {
			BeforeEach(func() {
				expectedJSON := notificationJSON("warning")
				header := http.Header{}
				header.Add("Retry-After", "5")

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
						ghttp.RespondWith(http.StatusTooManyRequests, nil, header),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("tries again after the time it was told", func() {
				done := make(chan struct{})

				go func() {
					defer GinkgoRecover()

					err := slackNotifier.SendNotification(
						logger,
						notifications.Notification{
							Owner:      "owner",
							Repository: "repo",
							Private:    private,
							SHA:        "abc123",
							Path:       "path/to/file.txt",
							LineNumber: 123,
						},
					)
					Expect(err).NotTo(HaveOccurred())

					close(done)
				}()

				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Consistently(done).ShouldNot(BeClosed())

				clock.IncrementBySeconds(4)

				Consistently(server.ReceivedRequests).Should(HaveLen(1))
				Consistently(done).ShouldNot(BeClosed())

				clock.IncrementBySeconds(2) // 6 seconds total

				Eventually(server.ReceivedRequests).Should(HaveLen(2))
				Eventually(done).Should(BeClosed())
			})
		})

		Context("when Slack responds with an 429 Too Many Requests more than 3 times", func() {
			BeforeEach(func() {
				expectedJSON := notificationJSON("warning")
				header := http.Header{}
				header.Add("Retry-After", "5")

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
						ghttp.RespondWith(http.StatusTooManyRequests, nil, header),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
						ghttp.RespondWith(http.StatusTooManyRequests, nil, header),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
						ghttp.RespondWith(http.StatusTooManyRequests, nil, header),
					),
				)
			})

			It("gives up and returns an error", func() {
				done := make(chan struct{})

				go func() {
					defer GinkgoRecover()

					err := slackNotifier.SendNotification(
						logger,
						notifications.Notification{
							Owner:      "owner",
							Repository: "repo",
							Private:    private,
							SHA:        "abc123",
							Path:       "path/to/file.txt",
							LineNumber: 123,
						},
					)
					Expect(err).To(HaveOccurred())

					close(done)
				}()

				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Eventually(done).ShouldNot(BeClosed())

				clock.IncrementBySeconds(6)

				Eventually(server.ReceivedRequests).Should(HaveLen(2))
				Consistently(done).ShouldNot(BeClosed())

				clock.IncrementBySeconds(6)

				Eventually(server.ReceivedRequests).Should(HaveLen(3))
				Eventually(done).Should(BeClosed())
			})
		})

		Context("when the repo is public", func() {
			BeforeEach(func() {
				private = false
			})

			It("notifies with the danger color", func() {
				expectedJSON := notificationJSON("danger")
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)

				err := slackNotifier.SendNotification(
					logger,
					notifications.Notification{
						Owner:      "owner",
						Repository: "repo",
						Private:    private,
						SHA:        "abc123",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})
	})

})

func notificationJSON(color string) string {
	return fmt.Sprintf(`{
				"attachments": [
					{
						"title": "Credential detected in owner/repo!",
						"text": "<https://github.com/owner/repo/blob/abc123/path/to/file.txt#L123|path/to/file.txt:123>",
						"color": "%s",
						"fallback": "https://github.com/owner/repo/blob/abc123/path/to/file.txt#L123"
					}
				]
			}
			`,
		color)
}

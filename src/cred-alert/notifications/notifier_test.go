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
				SHA:        "12345abcdef",
				Path:       "a/path/to/a/file",
				LineNumber: 42,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("sending batch slack notifications", func() {
		var (
			server *ghttp.Server

			batch   []notifications.Notification
			sendErr error
		)

		BeforeEach(func() {
			server = ghttp.NewServer()
			slackNotifier = notifications.NewSlackNotifier(server.URL(), clock)
		})

		AfterEach(func() {
			server.Close()
		})

		JustBeforeEach(func() {
			sendErr = slackNotifier.SendBatchNotification(logger, batch)
		})

		Context("when there is none notification in the batch", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{}
			})

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("doesn't send anything to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(0))
			})
		})

		Context("when there is one notifications in the batch", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
				}

				commitLink := "https://github.com/owner/repo/commit/abc1234567890"
				fileLink := "https://github.com/owner/repo/blob/abc1234567890/path/to/file.txt"
				lineLink := fmt.Sprintf("%s#L123", fileLink)
				expectedJSON := fmt.Sprintf(`
				{
					"attachments": [
						{
							"title": "Possible credentials found in <%s|owner/repo / abc1234>!",
							"text": "• <%s|path/to/file.txt> on line <%s|123>",
							"color": "danger",
							"fallback": "Possible credentials found in %s!"
						}
					]
				}
				`, commitLink, fileLink, lineLink, commitLink)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when there are multiple notifications in the batch in the same file", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 346,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 3932,
					},
				}

				commitLink := "https://github.com/owner/repo/commit/abc1234567890"
				fileLink := "https://github.com/owner/repo/blob/abc1234567890/path/to/file.txt"
				lineLink := fmt.Sprintf("%s#L123", fileLink)
				otherLineLink := fmt.Sprintf("%s#L346", fileLink)
				yetAnotherLineLink := fmt.Sprintf("%s#L3932", fileLink)

				expectedJSON := fmt.Sprintf(`
				{
					"attachments": [
						{
							"title": "Possible credentials found in <%s|owner/repo / abc1234>!",
							"text": "• <%s|path/to/file.txt> on lines <%s|123>, <%s|346>, and <%s|3932>",
							"color": "danger",
							"fallback": "Possible credentials found in %s!"
						}
					]
				}
				`, commitLink, fileLink, lineLink, otherLineLink, yetAnotherLineLink, commitLink)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when there are multiple notifications in the batch in different files", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file2.txt",
						LineNumber: 346,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 3932,
					},
				}

				commitLink := "https://github.com/owner/repo/commit/abc1234567890"
				fileLink := "https://github.com/owner/repo/blob/abc1234567890/path/to/file.txt"
				otherFileLink := "https://github.com/owner/repo/blob/abc1234567890/path/to/file2.txt"
				lineLink := fmt.Sprintf("%s#L123", fileLink)
				otherLineLink := fmt.Sprintf("%s#L346", otherFileLink)
				yetAnotherLineLink := fmt.Sprintf("%s#L3932", fileLink)

				expectedJSON := fmt.Sprintf(`
				{
					"attachments": [
						{
							"title": "Possible credentials found in <%s|owner/repo / abc1234>!",
							"text": "• <%s|path/to/file.txt> on lines <%s|123> and <%s|3932>\n• <%s|path/to/file2.txt> on line <%s|346>",
							"color": "danger",
							"fallback": "Possible credentials found in %s!"
						}
					]
				}
				`, commitLink, fileLink, lineLink, yetAnotherLineLink, otherFileLink, otherLineLink, commitLink)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
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
				SHA:        "abc123456",
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
							SHA:        "abc123456",
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
							SHA:        "abc123456",
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
						SHA:        "abc123456",
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
						"title": "Possible credentials found in <https://github.com/owner/repo/commit/abc123456|owner/repo / abc1234>!",
						"text": "• <https://github.com/owner/repo/blob/abc123456/path/to/file.txt|path/to/file.txt> on line <https://github.com/owner/repo/blob/abc123456/path/to/file.txt#L123|123>",
						"color": "%s",
						"fallback": "Possible credentials found in https://github.com/owner/repo/commit/abc123456!"
					}
				]
			}
			`,
		color)
}

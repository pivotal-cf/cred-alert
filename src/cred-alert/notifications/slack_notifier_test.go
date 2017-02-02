package notifications_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/ghttp"

	"cred-alert/notifications"
	"cred-alert/notifications/notificationsfakes"
)

const expectedJSON = `
{
  "attachments": [
    {
      "fallback": "",
      "color": "",
      "title": "",
      "text": "cool credential you have there, be a shame if something happened to it"
    }
  ]
}
`

var _ = Describe("SlackNotifier", func() {
	var (
		notifier notifications.Notifier

		clock  *fakeclock.FakeClock
		server *ghttp.Server
		logger *lagertest.TestLogger

		whitelist      notifications.Whitelist
		whitelistRules []string

		batch   []notifications.Notification
		sendErr error

		formatter *notificationsfakes.FakeSlackNotificationFormatter
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		whitelistRules = []string{}

		clock = fakeclock.NewFakeClock(time.Now())

		formatter = &notificationsfakes.FakeSlackNotificationFormatter{}
		formatter.FormatNotificationsStub = func(batch []notifications.Notification) []notifications.SlackMessage {
			if len(batch) == 0 {
				return []notifications.SlackMessage{}
			}

			return []notifications.SlackMessage{
				{
					Attachments: []notifications.SlackAttachment{
						{
							Text: "cool credential you have there, be a shame if something happened to it",
						},
					},
				},
			}
		}
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		whitelist = notifications.BuildWhitelist(whitelistRules...)
		notifier = notifications.NewSlackNotifier(server.URL(), clock, whitelist, formatter)
		logger = lagertest.NewTestLogger("slack-notifier")
	})

	Describe("SendBatchNotification", func() {
		JustBeforeEach(func() {
			sendErr = notifier.SendBatchNotification(logger, batch)
		})

		Context("when no notifications are given", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{}
			})

			It("does not return an error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("doesn't send anything to the server", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(0))
			})
		})

		Context("when there is one private notification in the batch", func() {
			BeforeEach(func() {
				batch = []notifications.Notification{
					{
						Owner:      "owner",
						Repository: "repo",
						Private:    true,
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
				}

				server.AppendHandlers(
					ghttp.VerifyRequest("POST", "/"),
				)
			})

			It("does not return an error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})

			Context("when the notification matches a white listed repository", func() {
				BeforeEach(func() {
					whitelistRules = []string{".*repo.*"}
				})

				It("doesn't send anything to slack", func() {
					Expect(server.ReceivedRequests()).Should(BeEmpty())
				})
			})
		})

		Context("when the notification matches a public white listed repository", func() {
			BeforeEach(func() {
				whitelistRules = []string{".*repo.*"}

				batch = []notifications.Notification{
					{
						Owner:      "owner",
						Repository: "repo",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
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
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 346,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 3932,
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("does not return an error", func() {
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
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 123,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						SHA:        "abc1234567890",
						Path:       "path/to/file2.txt",
						LineNumber: 346,
					},
					{
						Owner:      "owner",
						Repository: "repo",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 3932,
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("does not return an error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})
	})

	Describe("sending slack notifications", func() {
		Context("when the server responds successfully on the first try", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("only makes one request", func() {
				notification := notifications.Notification{
					Owner:      "owner",
					Repository: "repo",
					Private:    true,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				}
				err := notifier.SendNotification(logger, notification)
				Expect(err).NotTo(HaveOccurred())

				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when the server responds with an 429 Too Many Requests", func() {
			BeforeEach(func() {
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

					err := notifier.SendNotification(
						logger,
						notifications.Notification{
							Owner:      "owner",
							Repository: "repo",
							Private:    true,
							SHA:        "abc1234567890",
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

					err := notifier.SendNotification(
						logger,
						notifications.Notification{
							Owner:      "owner",
							Repository: "repo",
							Private:    true,
							SHA:        "abc1234567890",
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
	})
})

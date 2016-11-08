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
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		whitelistRules = []string{}
		clock = fakeclock.NewFakeClock(time.Now())
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		whitelist = notifications.BuildWhitelist(whitelistRules...)
		notifier = notifications.NewSlackNotifier(server.URL(), clock, whitelist)
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
			var expectedJSON string

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

				expectedJSON = calculateExpectedJSON(batch[0])

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

				expectedJSON := calculateExpectedJSON(batch[0])

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

			It("does not return an error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when there are multiple notifications in the batch in different repositories", func() {
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
						Repository: "repo2",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 346,
					},
					{
						Owner:      "owner",
						Repository: "repo3",
						SHA:        "abc1234567890",
						Path:       "path/to/file.txt",
						LineNumber: 3932,
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(calculateExpectedJSON(batch[0])),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(calculateExpectedJSON(batch[1])),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(calculateExpectedJSON(batch[2])),
					),
				)
			})

			It("does not return an error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(3))
			})
		})
	})

	Describe("sending slack notifications", func() {
		Context("when the server responds successfully on the first try", func() {
			BeforeEach(func() {
				expectedJSON := calculateExpectedJSON(notifications.Notification{
					Owner:      "owner",
					Repository: "repo",
					Private:    true,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				})

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
				expectedJSON := calculateExpectedJSON(notifications.Notification{
					Owner:      "owner",
					Repository: "repo",
					Private:    true,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				})

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
				expectedJSON := calculateExpectedJSON(notifications.Notification{
					Owner:      "owner",
					Repository: "repo",
					Private:    true,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				})

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

		Context("when the repo is public", func() {
			BeforeEach(func() {
				expectedJSON := calculateExpectedJSON(notifications.Notification{
					Owner:      "owner",
					Repository: "repo",
					Private:    false,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				})
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("notifies with the danger color", func() {
				err := notifier.SendNotification(
					logger,
					notifications.Notification{
						Owner:      "owner",
						Repository: "repo",
						Private:    false,
						SHA:        "abc1234567890",
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

func calculateExpectedJSON(notification notifications.Notification) string {
	commitLink := fmt.Sprintf(
		"https://github.com/%s/%s/commit/%s",
		notification.Owner,
		notification.Repository,
		notification.SHA,
	)
	fileLink := fmt.Sprintf(
		"https://github.com/%s/%s/blob/%s/%s",
		notification.Owner,
		notification.Repository,
		notification.SHA,
		notification.Path,
	)
	lineLink := fmt.Sprintf("%s#L%d", fileLink, notification.LineNumber)
	color := "danger"
	if notification.Private {
		color = "warning"
	}

	return fmt.Sprintf(`
				{
					"attachments": [
						{
							"title": "Possible credentials found in <%s|%s/%s / abc1234>!",
							"text": "• <%s|path/to/file.txt> on line <%s|%d>",
							"color": "%s",
							"fallback": "Possible credentials found in %s!"
						}
					]
				}
				`, commitLink, notification.Owner, notification.Repository, fileLink,
		lineLink, notification.LineNumber, color, commitLink)
}

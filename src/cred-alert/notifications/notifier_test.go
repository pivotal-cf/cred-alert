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

		clock  *fakeclock.FakeClock
		server *ghttp.Server
		logger *lagertest.TestLogger

		whitelist      notifications.Whitelist
		whitelistRules []string

		batch   []notifications.Notification
		sendErr error
	)

	//==================================================================================

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
		slackNotifier = notifications.NewSlackNotifier(server.URL(), clock, whitelist)
		logger = lagertest.NewTestLogger("slack-notifier")
	})

	appendVerifyJson := func(expectedJSON string) {
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/"),
				ghttp.VerifyJSON(expectedJSON),
			),
		)
	}

	//=================================================================================

	Describe("with an empty webhook URL", func() {
		JustBeforeEach(func() {
			slackNotifier = notifications.NewSlackNotifier("", clock, whitelist)
		})

		It("returns a nullNotifier", func() {
			Expect(slackNotifier).NotTo(BeNil())
		})

		It("handles sending notifications", func() {
			err := slackNotifier.SendNotification(logger, createNotificationType1("repo", true, "file", 42))

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("sending batch slack notifications", func() {
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

		Context("when there is one private notification in the batch", func() {
			var expectedJSON string

			BeforeEach(func() {
				batch = []notifications.Notification{
					createNotificationType1("repo", true, "file.txt", 123),
				}

				expectedJSON = calculateExpectedJSON(batch[0])

				appendVerifyJson(expectedJSON)
			})

			Context("single warning response", func() {
				It("does not error", func() {
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
		})

		Context("when the notification matches a public white listed repository", func() {
			BeforeEach(func() {
				whitelistRules = []string{".*repo.*"}

				batch = []notifications.Notification{
					createNotificationType1("repo", false, "file.txt", 123),
				}

				expectedJSON := calculateExpectedJSON(batch[0])

				appendVerifyJson(expectedJSON)
			})

			It("sends a message to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when there are multiple notifications in the batch in the same file", func() {
			BeforeEach(func() {
				batch = createBatch1()

				commitLink := "https://github.com/owner/repo/commit/abc1234567890"
				fileLink := createFileLink1("path/to/file.txt")
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

				appendVerifyJson(expectedJSON)

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
				batch = createBatch2()

				commitLink := "https://github.com/owner/repo/commit/abc1234567890"
				fileLink := createFileLink1("path/to/file.txt")
				otherFileLink := createFileLink1("path/to/file2.txt")
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

				appendVerifyJson(expectedJSON)
			})

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when there are multiple notifications in the batch in different repositories", func() {
			BeforeEach(func() {
				batch = createBatch3()

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

			It("does not error", func() {
				Expect(sendErr).NotTo(HaveOccurred())
			})

			It("sends a message with all of them in to slack", func() {
				Expect(server.ReceivedRequests()).Should(HaveLen(3))
			})
		})
	})

	Describe("sending slack notifications", func() {
		Context("when everything goes to plan", func() {
			BeforeEach(func() {
				expectedJSON := notificationJSON(true)

				appendVerifyJson(expectedJSON)

			})

			It("POSTs a message to the fake slack webhook", func() {
				err := slackNotifier.SendNotification(logger, createNotificationType1("repo", true, "file.txt", 123))
				Expect(err).NotTo(HaveOccurred())

				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when Slack responds with an 429 Too Many Requests", func() {
			BeforeEach(func() {
				expectedJSON := notificationJSON(true)
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
						createNotificationType1("repo", true, "file.txt", 123),
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
				expectedJSON := notificationJSON(true)
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
						createNotificationType1("repo", true, "file.txt", 123),
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
				expectedJSON := notificationJSON(false)
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSON(expectedJSON),
					),
				)
			})

			It("notifies with the danger color", func() {
				err := slackNotifier.SendNotification(
					logger,
					createNotificationType1("repo", false, "file.txt", 123),
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})
	})

})

//========================================================================================================

//========================================================================================================
//Sample test data functions

func createNotificationType1(repo string, isPrivate bool, file string, lineNumber int) notifications.Notification {
	return notifications.Notification{
		Owner:      "owner",
		Repository: repo,
		Private:    isPrivate,
		SHA:        "abc1234567890",
		Path:       "path/to/" + file,
		LineNumber: lineNumber,
	}
}

func createBatchType1(repo2, file2 string, repo3, file3 string) []notifications.Notification {

	batch := []notifications.Notification{
		createNotificationType1("repo", false, "file.txt", 123),
		createNotificationType1(repo2, false, file2, 346),
		createNotificationType1(repo3, false, file3, 3932),
	}

	return batch
}

func createBatch1() []notifications.Notification {
	return createBatchType1("repo", "file.txt", "repo", "file.txt")
}

func createBatch2() []notifications.Notification {
	return createBatchType1("repo", "file2.txt", "repo", "file.txt")
}

func createBatch3() []notifications.Notification {
	return createBatchType1("repo2", "file.txt", "repo3", "file.txt")
}

func createFileLink1(path string) string {
	return "https://github.com/owner/repo/blob/abc1234567890/" + path
}

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

func notificationJSON(isPrivate bool) string {
	return calculateExpectedJSON(notifications.Notification{
		Owner:      "owner",
		Repository: "repo",
		Private:    isPrivate,
		SHA:        "abc1234567890",
		Path:       "path/to/file.txt",
		LineNumber: 123,
	})
}

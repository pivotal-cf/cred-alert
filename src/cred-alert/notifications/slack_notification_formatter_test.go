package notifications_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/notifications"
)

var _ = Describe("Formatting Slack notifications", func() {
	var (
		formatter notifications.SlackNotificationFormatter
	)

	BeforeEach(func() {
		formatter = notifications.NewSlackNotificationFormatter()
	})

	Context("when there are multiple notifications for the same file", func() {
		It("keeps the message short by putting them on the same line", func() {
			batch := []notifications.Notification{
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

			formatted := formatter.FormatNotifications(batch)

			Expect(formatted).To(HaveLen(1))
			Expect(formatted[0].Attachments).To(HaveLen(1))

			attachment := formatted[0].Attachments[0]

			Expect(attachment.Title).To(Equal(fmt.Sprintf("Possible credentials found in <%s|owner/repo / abc1234>!", commitLink)))
			Expect(attachment.Color).To(Equal("danger"))
			Expect(attachment.Fallback).To(Equal(fmt.Sprintf("Possible credentials found in %s!", commitLink)))

			Expect(attachment.Text).To(ContainSubstring(fileLink))
			Expect(attachment.Text).To(ContainSubstring(lineLink))
			Expect(attachment.Text).To(ContainSubstring(otherLineLink))
			Expect(attachment.Text).To(ContainSubstring(yetAnotherLineLink))
		})
	})

	Context("when there are multiple notifications for a different files", func() {
		It("puts them in different sections", func() {
			batch := []notifications.Notification{
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

			formatted := formatter.FormatNotifications(batch)

			Expect(formatted).To(HaveLen(1))
			Expect(formatted[0].Attachments).To(HaveLen(1))

			attachment := formatted[0].Attachments[0]

			Expect(attachment.Title).To(Equal(fmt.Sprintf("Possible credentials found in <%s|owner/repo / abc1234>!", commitLink)))
			Expect(attachment.Color).To(Equal("danger"))
			Expect(attachment.Fallback).To(Equal(fmt.Sprintf("Possible credentials found in %s!", commitLink)))

			Expect(attachment.Text).To(ContainSubstring(fileLink))
			Expect(attachment.Text).To(ContainSubstring(otherFileLink))
			Expect(attachment.Text).To(ContainSubstring(lineLink))
			Expect(attachment.Text).To(ContainSubstring(otherLineLink))
			Expect(attachment.Text).To(ContainSubstring(yetAnotherLineLink))
		})
	})

	Context("when there are multiple notifications in the batch in different repositories", func() {
		It("sends a message with all of them in to slack", func() {
			batch := []notifications.Notification{
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

			commitLink := "https://github.com/owner/repo/commit/abc1234567890"
			commitLink2 := "https://github.com/owner/repo2/commit/abc1234567890"
			commitLink3 := "https://github.com/owner/repo3/commit/abc1234567890"

			fileLink := "https://github.com/owner/repo/blob/abc1234567890/path/to/file.txt"
			fileLink2 := "https://github.com/owner/repo2/blob/abc1234567890/path/to/file.txt"
			fileLink3 := "https://github.com/owner/repo3/blob/abc1234567890/path/to/file.txt"

			lineLink := fmt.Sprintf("%s#L123", fileLink)
			lineLink2 := fmt.Sprintf("%s#L346", fileLink2)
			lineLink3 := fmt.Sprintf("%s#L3932", fileLink3)

			expectedMessage := notifications.SlackMessage{
				Attachments: []notifications.SlackAttachment{
					{
						Title:    fmt.Sprintf("Possible credentials found in <%s|owner/repo / abc1234>!", commitLink),
						Text:     fmt.Sprintf("• <%s|path/to/file.txt> on line <%s|123>", fileLink, lineLink),
						Color:    "danger",
						Fallback: fmt.Sprintf("Possible credentials found in %s!", commitLink),
					},
				},
			}

			expectedMessage2 := notifications.SlackMessage{
				Attachments: []notifications.SlackAttachment{
					{
						Title:    fmt.Sprintf("Possible credentials found in <%s|owner/repo2 / abc1234>!", commitLink2),
						Text:     fmt.Sprintf("• <%s|path/to/file.txt> on line <%s|346>", fileLink2, lineLink2),
						Color:    "danger",
						Fallback: fmt.Sprintf("Possible credentials found in %s!", commitLink2),
					},
				},
			}

			expectedMessage3 := notifications.SlackMessage{
				Attachments: []notifications.SlackAttachment{
					{
						Title:    fmt.Sprintf("Possible credentials found in <%s|owner/repo3 / abc1234>!", commitLink3),
						Text:     fmt.Sprintf("• <%s|path/to/file.txt> on line <%s|3932>", fileLink3, lineLink3),
						Color:    "danger",
						Fallback: fmt.Sprintf("Possible credentials found in %s!", commitLink3),
					},
				},
			}

			Expect(formatter.FormatNotifications(batch)).To(ConsistOf(expectedMessage, expectedMessage2, expectedMessage3))
		})
	})

	Context("when the commit is in a private repository", func() {
		It("uses the warning color", func() {
			batch := []notifications.Notification{
				{
					Owner:      "owner",
					Repository: "repo",
					Private:    true,
					SHA:        "abc1234567890",
					Path:       "path/to/file.txt",
					LineNumber: 123,
				},
			}

			notifications := formatter.FormatNotifications(batch)
			Expect(notifications[0].Attachments[0].Color).To(Equal("warning"))
		})
	})
})

package notifications_test

import (
	"errors"
	"rolodex/rolodexpb"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"cred-alert/notifications"
	"cred-alert/notifications/notificationsfakes"
)

var _ = Describe("Rolodex", func() {
	var (
		client  *notificationsfakes.FakeRolodexClient
		mapping map[string]string

		logger *lagertest.TestLogger

		rolodex notifications.AddressBook
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("rolodex")
		client = &notificationsfakes.FakeRolodexClient{}
	})

	JustBeforeEach(func() {
		urls := notifications.NewTeamURLs("default.slack.example.com/webhook", "default", mapping)
		rolodex = notifications.NewRolodex(client, urls)
	})

	Describe("finding the URL and channel for a repository notification", func() {
		Context("when rolodex knows about a repository", func() {
			BeforeEach(func() {
				client.GetOwnersReturns(&rolodexpb.GetOwnersResponse{
					Teams: []*rolodexpb.Team{
						{
							Name: "sec-red",
							SlackChannel: &rolodexpb.SlackChannel{
								Team: "pivotal",
								Name: "sec-red",
							},
						},
						{
							Name: "sec-blue",
							SlackChannel: &rolodexpb.SlackChannel{
								Team: "pivotal-cf",
								Name: "sec-blue",
							},
						},
					},
				}, nil)
			})

			Context("when there is a webhook URL configured for that team", func() {
				BeforeEach(func() {
					mapping = map[string]string{
						"pivotal":    "pivotal.slack.example.com/webhook",
						"pivotal-cf": "pivotal-cf.slack.example.com/webhook",
					}
				})

				It("asks for the correct repository", func() {
					_ = rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")
					Expect(client.GetOwnersCallCount()).To(Equal(1))

					ctx, clientRequest, _ := client.GetOwnersArgsForCall(0)
					Expect(clientRequest.Repository.Owner).To(Equal("pivotal-cf"))
					Expect(clientRequest.Repository.Name).To(Equal("cred-alert"))

					_, set := ctx.Deadline()
					Expect(set).To(BeTrue())
				})

				It("returns the addresses of the notification channel", func() {
					addresses := rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")

					Expect(addresses).To(HaveLen(2))

					address := addresses[0]
					Expect(address.URL).To(Equal("pivotal.slack.example.com/webhook"))
					Expect(address.Channel).To(Equal("sec-red"))

					address = addresses[1]
					Expect(address.URL).To(Equal("pivotal-cf.slack.example.com/webhook"))
					Expect(address.Channel).To(Equal("sec-blue"))
				})
			})

			Context("when there is not a webhook URL configured for that team", func() {
				BeforeEach(func() {
					mapping = map[string]string{
						"pivotal-cf": "pivotal-cf.slack.example.com/webhook",
					}
				})

				It("asks for the correct repository", func() {
					_ = rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")
					Expect(client.GetOwnersCallCount()).To(Equal(1))

					ctx, clientRequest, _ := client.GetOwnersArgsForCall(0)
					Expect(clientRequest.Repository.Owner).To(Equal("pivotal-cf"))
					Expect(clientRequest.Repository.Name).To(Equal("cred-alert"))

					_, set := ctx.Deadline()
					Expect(set).To(BeTrue())
				})

				It("returns the default address", func() {
					addresses := rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")

					Expect(addresses).To(HaveLen(2))

					address := addresses[0]
					Expect(address.URL).To(Equal("default.slack.example.com/webhook"))
					Expect(address.Channel).To(Equal("default"))

					address = addresses[1]
					Expect(address.URL).To(Equal("pivotal-cf.slack.example.com/webhook"))
					Expect(address.Channel).To(Equal("sec-blue"))
				})
			})
		})

		Context("when rolodex returns no teams", func() {
			BeforeEach(func() {
				client.GetOwnersReturns(&rolodexpb.GetOwnersResponse{
					Teams: []*rolodexpb.Team{},
				}, nil)
				mapping = map[string]string{
					"pivotal": "pivotal.slack.example.com/webhook",
				}
			})

			It("returns the default channel", func() {
				addresses := rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")

				Expect(addresses).To(HaveLen(1))

				address := addresses[0]
				Expect(address.URL).To(Equal("default.slack.example.com/webhook"))
				Expect(address.Channel).To(Equal("default"))
			})
		})

		Context("when rolodex returns an error", func() {
			BeforeEach(func() {
				client.GetOwnersReturns(nil, errors.New("disaster"))
				mapping = map[string]string{
					"pivotal": "pivotal.slack.example.com/webhook",
				}
			})

			It("logs the error", func() {
				_ = rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")
				Expect(logger).To(gbytes.Say("error"))
			})

			It("returns the default channel even if the mapping existed", func() {
				addresses := rolodex.AddressForRepo(logger, "pivotal-cf", "cred-alert")

				Expect(addresses).To(HaveLen(1))

				address := addresses[0]
				Expect(address.URL).To(Equal("default.slack.example.com/webhook"))
				Expect(address.Channel).To(Equal("default"))
			})
		})
	})
})

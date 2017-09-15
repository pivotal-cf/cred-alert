package notifications_test

import (
	"fmt"

	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/notifications"
	"cred-alert/notifications/notificationsfakes"
)

var _ = Describe("Router", func() {
	var (
		router notifications.Router

		notifier    *notificationsfakes.FakeNotifier
		addressBook *notificationsfakes.FakeAddressBook
		whitelist   *notificationsfakes.FakeWhitelist
		logger      *lagertest.TestLogger
	)

	BeforeEach(func() {
		notifier = &notificationsfakes.FakeNotifier{}
		addressBook = &notificationsfakes.FakeAddressBook{}
		whitelist = &notificationsfakes.FakeWhitelist{}

		logger = lagertest.NewTestLogger("router")

		router = notifications.NewRouter(notifier, addressBook, whitelist)
	})

	Describe("Deliver", func() {
		It("groups notifications into envelopes and sends them", func() {
			addressBook.AddressForRepoStub = func(_ context.Context, _ lager.Logger, isPrivate bool, owner, name string) []notifications.Address {
				repo := fmt.Sprintf("%s/%s", owner, name)

				switch repo {
				case "pivotal-cf/cred-alert":
					return []notifications.Address{
						{
							URL:     "https://a.example.com",
							Channel: "channel-a",
						},
						{
							URL:     "https://a.example.com",
							Channel: "channel-b",
						},
					}
				case "pivotal-cf/scantron":
					return []notifications.Address{{
						URL:     "https://a.example.com",
						Channel: "channel-b",
					}}
				case "pivotal-cf/credhub":
					return []notifications.Address{{
						URL:     "https://b.example.com",
						Channel: "channel-a",
					}}
				default:
					panic("I don't know about a repository called " + repo + "!")
				}
			}

			note1 := notifications.Notification{
				Owner:      "pivotal-cf",
				Repository: "cred-alert",
				Path:       "some/path/1",
			}

			note2 := notifications.Notification{
				Owner:      "pivotal-cf",
				Repository: "scantron",
				Path:       "some/path/2",
			}

			note3 := notifications.Notification{
				Owner:      "pivotal-cf",
				Repository: "credhub",
				Path:       "some/path/3",
			}

			router.Deliver(context.Background(), logger, []notifications.Notification{note1, note2, note3})

			Expect(notifier.SendCallCount()).To(Equal(3))

			_, _, envelope := notifier.SendArgsForCall(0)
			Expect(envelope.Address.URL).To(Equal("https://a.example.com"))
			Expect(envelope.Address.Channel).To(Equal("channel-a"))
			Expect(envelope.Contents).To(ConsistOf(note1))

			_, _, envelope = notifier.SendArgsForCall(1)
			Expect(envelope.Address.URL).To(Equal("https://a.example.com"))
			Expect(envelope.Address.Channel).To(Equal("channel-b"))
			Expect(envelope.Contents).To(ConsistOf(note1, note2))

			_, _, envelope = notifier.SendArgsForCall(2)
			Expect(envelope.Address.URL).To(Equal("https://b.example.com"))
			Expect(envelope.Address.Channel).To(Equal("channel-a"))
			Expect(envelope.Contents).To(ConsistOf(note3))
		})

		It("skips whitelisted repositories", func() {
			addressBook.AddressForRepoReturns([]notifications.Address{{
				URL:     "https://example.com",
				Channel: "some-channel",
			}})

			whitelist.ShouldSkipNotificationStub = func(_ bool, name string) bool {
				return name == "whitelisted-repo"
			}

			note1 := notifications.Notification{
				Owner:      "pivotal-cf",
				Repository: "cred-alert",
				Private:    false,
				Path:       "some/path/1",
			}

			note2 := notifications.Notification{
				Owner:      "pivotal-cf",
				Repository: "whitelisted-repo",
				Private:    true,
				Path:       "some/path/2",
			}

			router.Deliver(context.Background(), logger, []notifications.Notification{note1, note2})

			Expect(notifier.SendCallCount()).To(Equal(1))

			_, _, envelope := notifier.SendArgsForCall(0)
			Expect(envelope.Contents).To(ConsistOf(note1))

			Expect(whitelist.ShouldSkipNotificationCallCount()).To(Equal(2))

			private, repo := whitelist.ShouldSkipNotificationArgsForCall(0)
			Expect(private).To(BeFalse())
			Expect(repo).To(Equal("cred-alert"))

			private, repo = whitelist.ShouldSkipNotificationArgsForCall(1)
			Expect(private).To(BeTrue())
			Expect(repo).To(Equal("whitelisted-repo"))
		})
	})
})

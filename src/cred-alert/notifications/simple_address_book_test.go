package notifications_test

import (
	"cred-alert/notifications"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Simple Address Book", func() {
	var (
		addressBook notifications.AddressBook
		logger      *lagertest.TestLogger
	)

	BeforeEach(func() {
		addressBook = notifications.NewSimpleAddressBook(
			"https://example.com",
			"some-channel",
		)

		logger = lagertest.NewTestLogger("simple-address-book")
	})

	It("returns the same address for every repo", func() {
		addresses1 := addressBook.AddressForRepo(context.Background(), logger, false, "some-owner", "some-repo")
		addresses2 := addressBook.AddressForRepo(context.Background(), logger, false, "other-owner", "other-repo")

		Expect(addresses1).To(Equal(addresses2))

		Expect(addresses1).To(HaveLen(1))
		Expect(addresses1[0].URL).To(Equal("https://example.com"))
		Expect(addresses1[0].Channel).To(Equal("some-channel"))
	})
})

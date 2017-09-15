package notifications

import (
	"context"

	"code.cloudfoundry.org/lager"
)

type simpleAddressBook struct {
	url     string
	channel string
}

func NewSimpleAddressBook(url, channel string) AddressBook {
	return &simpleAddressBook{
		url:     url,
		channel: channel,
	}
}

func (s *simpleAddressBook) AddressForRepo(ctx context.Context, logger lager.Logger, isPrivate bool, owner, name string) []Address {
	return []Address{{
		URL:     s.url,
		Channel: s.channel,
	}}
}

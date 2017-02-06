package notifications

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . Router

type Router interface {
	Deliver(logger lager.Logger, batch []Notification) error
}

type router struct {
	notifier    Notifier
	addressBook AddressBook
	whitelist   Whitelist
}

func NewRouter(notifier Notifier, addressBook AddressBook, whitelist Whitelist) Router {
	return &router{
		notifier:    notifier,
		addressBook: addressBook,
		whitelist:   whitelist,
	}
}

func (r *router) Deliver(logger lager.Logger, batch []Notification) error {
	logger = logger.Session("deliver")

	envelopes := r.filterAndGroupByDestination(logger, batch)

	for _, envelope := range envelopes {
		_ = r.notifier.Send(logger, *envelope)
	}

	return nil
}

func (r *router) filterAndGroupByDestination(logger lager.Logger, batch []Notification) []*Envelope {
	bag := mailbag{}

	for _, notification := range batch {
		if r.whitelist.ShouldSkipNotification(notification.Private, notification.Repository) {
			continue
		}

		addresses := r.addressBook.AddressForRepo(logger, notification.Owner, notification.Repository)

		for _, address := range addresses {
			bag.envelopeToAddress(notification, address)
		}
	}

	return bag.Envelopes
}

type mailbag struct {
	Envelopes []*Envelope
}

func (m *mailbag) envelopeToAddress(notification Notification, address Address) {
	for _, envelope := range m.Envelopes {
		if envelope.Address == address {
			envelope.Contents = append(envelope.Contents, notification)
			return
		}
	}

	envelope := &Envelope{
		Address:  address,
		Contents: []Notification{notification},
	}

	m.Envelopes = append(m.Envelopes, envelope)
}

package notifications

import (
	"context"

	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Router

type Router interface {
	Deliver(ctx context.Context, logger lager.Logger, batch []Notification) error
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

func (r *router) Deliver(ctx context.Context, logger lager.Logger, batch []Notification) error {
	logger = logger.Session("deliver")

	span := trace.FromContext(ctx).NewChild("notify")
	defer span.Finish()

	envelopes := r.filterAndGroupByDestination(ctx, logger, batch)

	logger.Debug("sending", lager.Data{
		"envelope-count":     len(envelopes),
		"notification-count": len(batch),
	})

	for _, envelope := range envelopes {
		err := r.notifier.Send(logger, *envelope)
		if err != nil {
			return err
		}
	}

	logger.Debug("sent")

	return nil
}

func (r *router) filterAndGroupByDestination(ctx context.Context, logger lager.Logger, batch []Notification) []*Envelope {
	bag := mailbag{}

	for _, notification := range batch {
		if r.whitelist.ShouldSkipNotification(notification.Private, notification.Repository) {
			continue
		}

		addresses := r.addressBook.AddressForRepo(ctx, logger, notification.Owner, notification.Repository)

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

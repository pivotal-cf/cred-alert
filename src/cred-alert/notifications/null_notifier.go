package notifications

import "code.cloudfoundry.org/lager"

type nullNotifier struct{}

func NewNullNotifier() Notifier {
	return &nullNotifier{}
}

func (n *nullNotifier) SendNotification(logger lager.Logger, notification Notification) error {
	return nil
}

func (n *nullNotifier) SendBatchNotification(logger lager.Logger, batch []Notification) error {
	return nil
}

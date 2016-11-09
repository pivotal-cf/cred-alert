package notifications

import (
	"fmt"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	SendNotification(lager.Logger, Notification) error
	SendBatchNotification(lager.Logger, []Notification) error
}

type Notification struct {
	Owner      string
	Repository string
	Private    bool

	Branch string
	SHA    string

	Path       string
	LineNumber int
}

func (n Notification) FullName() string {
	return fmt.Sprintf("%s/%s", n.Owner, n.Repository)
}

func (n Notification) ShortSHA() string {
	return n.SHA[:7]
}

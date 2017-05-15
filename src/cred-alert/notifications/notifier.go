package notifications

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Notifier

type Notifier interface {
	Send(context.Context, lager.Logger, Envelope) error
}

type Envelope struct {
	Address  Address
	Contents []Notification
}

func (e Envelope) Size() int {
	return len(e.Contents)
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

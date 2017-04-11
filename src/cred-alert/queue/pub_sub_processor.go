package queue

import (
	"context"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . PubSubProcessor

type PubSubProcessor interface {
	Process(context.Context, lager.Logger, *pubsub.Message) (bool, error)
}

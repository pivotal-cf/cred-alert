package queue

import (
	"context"

	"cloud.google.com/go/pubsub"
)

//go:generate counterfeiter . PubSubProcessor

type PubSubProcessor interface {
	Process(context.Context, *pubsub.Message) (bool, error)
}

package queue

import (
	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . PubSubProcessor

type PubSubProcessor interface {
	Process(lager.Logger, *pubsub.Message) (bool, error)
}

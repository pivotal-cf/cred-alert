package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

//go:generate counterfeiter . PubSubProcessor

type PubSubProcessor interface {
	Process(lager.Logger, *pubsub.Message) (bool, error)
}

type pubSubSubscriber struct {
	logger       lager.Logger
	subscription *pubsub.Subscription
	handler      RetryHandler
}

func NewPubSubSubscriber(logger lager.Logger, subscription *pubsub.Subscription, handler RetryHandler) ifrit.Runner {
	return &pubSubSubscriber{
		logger: logger.Session("message-processor", lager.Data{
			"subscription": subscription.ID(),
		}),
		subscription: subscription,
		handler:      handler,
	}
}

func (p *pubSubSubscriber) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	p.logger.Info("starting")
	it, err := p.subscription.Pull(context.Background())
	if err != nil {
		return err
	}

	finished := make(chan error)

	close(ready)
	p.logger.Info("started")

	go func() {
		for {
			message, err := it.Next()
			if err == iterator.Done {
				break
			}

			if err != nil {
				continue
			}

			logger := p.logger.Session("processing-message", lager.Data{
				"pubsub-message": message.ID,
			})

			p.handler.ProcessMessage(logger, message)
		}

		close(finished)
	}()

	<-signals
	p.logger.Info("told-to-exit")
	it.Stop()
	<-finished
	p.logger.Info("done")
	return nil
}

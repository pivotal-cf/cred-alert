package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

//go:generate counterfeiter . Handler

type Handler interface {
	ProcessMessage(*pubsub.Message) (bool, error)
}

type pubsubProcessor struct {
	logger       lager.Logger
	subscription *pubsub.Subscription
	handler      Handler
}

func NewProcessor(logger lager.Logger, subscription *pubsub.Subscription, handler Handler) ifrit.Runner {
	return &pubsubProcessor{
		logger:       logger,
		subscription: subscription,
		handler:      handler,
	}
}

func (p *pubsubProcessor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

			retryable, err := p.handler.ProcessMessage(message)
			if err != nil {
				p.logger.Error("failed-to-process-message", err)

				if retryable {
					message.Done(false)
				} else {
					message.Done(true)
				}

				continue
			}

			message.Done(true)
		}

		close(finished)
	}()

	select {
	case <-signals:
		p.logger.Info("received-exit-waiting-for-iterator")
		it.Stop()
		<-finished
		return nil
	}

	panic("unreachable")
}

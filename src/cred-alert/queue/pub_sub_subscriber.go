package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"

	"cred-alert/metrics"
)

type pubSubSubscriber struct {
	logger       lager.Logger
	subscription *pubsub.Subscription
	processor    PubSubProcessor

	processTimer          metrics.Timer
	processSuccessCounter metrics.Counter
	processFailureCounter metrics.Counter
	processRetryCounter   metrics.Counter
}

func NewPubSubSubscriber(
	logger lager.Logger,
	subscription *pubsub.Subscription,
	processor PubSubProcessor,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &pubSubSubscriber{
		logger: logger.Session("message-processor", lager.Data{
			"subscription": subscription.ID(),
		}),
		subscription:          subscription,
		processor:             processor,
		processTimer:          emitter.Timer("revok.pub_sub_subscriber.process.time"),
		processSuccessCounter: emitter.Counter("revok.pub_sub_subscriber.process.success"),
		processFailureCounter: emitter.Counter("revok.pub_sub_subscriber.process.failure"),
		processRetryCounter:   emitter.Counter("revok.pub_sub_subscriber.process.retries"),
	}
}

func (p *pubSubSubscriber) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	p.logger.Info("starting")
	it, err := p.subscription.Pull(context.Background())
	if err != nil {
		return err
	}

	p.logger.Info("started")

	finished := make(chan error)
	close(ready)

	go func() {
		for {
			message, err := it.Next()
			if err == iterator.Done {
				break
			}

			if err != nil {
				p.logger.Error("failed-to-pull-message", err)
				continue
			}

			logger := p.logger.Session("processing-message", lager.Data{
				"pubsub-message":      message.ID,
				"pubsub-publish-time": message.PublishTime.String(),
			})

			var retryable bool

			p.processTimer.Time(logger, func() {
				retryable, err = p.processor.Process(logger, message)
			})

			if err != nil {
				logger.Error("failed-to-process-message", err)

				if retryable {
					logger.Info("queuing-message-for-retry")
					message.Done(false)
					p.processRetryCounter.Inc(logger)
				} else {
					message.Done(true)
				}

				p.processFailureCounter.Inc(logger)
			} else {
				message.Done(true)
				p.processSuccessCounter.Inc(logger)
			}
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

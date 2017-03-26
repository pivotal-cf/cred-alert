package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"golang.org/x/net/context"

	"cred-alert/metrics"
)

const SubscriberCount = 2

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

	done := make(chan struct{})
	errs := make(chan error)
	cctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := p.subscription.Receive(cctx, func(ctx context.Context, message *pubsub.Message) {
			p.processMessage(ctx, message)
		})
		if err != nil {
			errs <- err
		}

		close(done)
	}()

	p.logger.Info("started")

	close(ready)

	select {
	case <-signals:
		p.logger.Info("signalled")
		cancel()
	case err := <-errs:
		p.logger.Error("failed", err)
		cancel()
		return err
	}

	<-done

	p.logger.Info("done")

	return nil
}

func (p *pubSubSubscriber) processMessage(ctx context.Context, message *pubsub.Message) {
	var (
		retryable bool
		err       error
	)

	logger := p.logger.Session("processing-message", lager.Data{
		"pubsub-message":      message.ID,
		"pubsub-publish-time": message.PublishTime.String(),
	})

	p.processTimer.Time(logger, func() {
		retryable, err = p.processor.Process(logger, message)
	})

	if err != nil {
		logger.Error("failed-to-process-message", err)

		if retryable {
			logger.Info("queuing-message-for-retry")
			message.Nack()
			p.processRetryCounter.Inc(logger)
		} else {
			message.Ack()
		}

		p.processFailureCounter.Inc(logger)
	} else {
		message.Ack()
		p.processSuccessCounter.Inc(logger)
	}
}

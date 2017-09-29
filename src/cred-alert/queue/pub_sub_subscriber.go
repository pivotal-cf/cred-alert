package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/tedsuo/ifrit"
	"golang.org/x/net/context"

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

		subscription: subscription,
		processor:    processor,

		processTimer:          emitter.Timer("revok.pub_sub_subscriber.process.time"),
		processSuccessCounter: emitter.Counter("revok.pub_sub_subscriber.process.success"),
		processFailureCounter: emitter.Counter("revok.pub_sub_subscriber.process.failure"),
		processRetryCounter:   emitter.Counter("revok.pub_sub_subscriber.process.retries"),
	}
}

func (p *pubSubSubscriber) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	p.logger.Info("starting")

	errs := make(chan error)
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.subscription.ReceiveSettings.MaxOutstandingMessages = 4

	go func() {
		errs <- p.subscription.Receive(cctx, func(ctx context.Context, message *pubsub.Message) {
			p.processMessage(ctx, message)
		})
	}()

	p.logger.Info("started")

	close(ready)

	select {
	case <-signals:
		p.logger.Info("signalled")
	case err := <-errs:
		if err != nil {
			p.logger.Error("failed", err)
			return err
		}
	}

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
	lctx := lagerctx.NewContext(ctx, logger)

	p.processTimer.Time(logger, func() {
		retryable, err = p.processor.Process(lctx, message)
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

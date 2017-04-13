package queue

import (
	"os"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/lager"
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

	traceClient *trace.Client
}

func NewPubSubSubscriber(
	logger lager.Logger,
	subscription *pubsub.Subscription,
	processor PubSubProcessor,
	emitter metrics.Emitter,
	traceClient *trace.Client,
) ifrit.Runner {
	return &pubSubSubscriber{
		logger: logger.Session("message-processor", lager.Data{
			"subscription": subscription.ID(),
		}),

		subscription: subscription,
		processor:    processor,
		traceClient:  traceClient,

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

	p.subscription.ReceiveSettings.MaxOutstandingMessages = 4

	go func() {
		errs <- p.subscription.Receive(cctx, func(ctx context.Context, message *pubsub.Message) {
			span := p.traceClient.NewSpan("PubsubReceive")
			span.SetLabel("Message", string(message.Data))
			defer span.Finish()

			ctx = trace.NewContext(ctx, span)

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

	cancel()

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
		retryable, err = p.processor.Process(ctx, logger, message)
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

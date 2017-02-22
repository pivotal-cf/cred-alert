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

type pubSubAcceptor struct {
	logger       lager.Logger
	subscription *pubsub.Subscription
	processor    PubSubProcessor

	processTimer   metrics.Timer
	processCounter metrics.Counter
}

func NewPubSubAcceptor(
	logger lager.Logger,
	subscription *pubsub.Subscription,
	processor PubSubProcessor,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &pubSubAcceptor{
		logger: logger.Session("message-acceptor", lager.Data{
			"subscription": subscription.ID(),
		}),
		subscription:   subscription,
		processor:      processor,
		processTimer:   emitter.Timer("revok.pubsub.process.time"),
		processCounter: emitter.Counter("revok.pubsub.process.count"),
	}
}

func (p *pubSubAcceptor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	p.logger.Info("starting")

	it, err := p.subscription.Pull(context.Background(), pubsub.MaxPrefetch(1))
	if err != nil {
		return err
	}

	p.logger.Info("started")
	close(ready)

	go func() {
		for {
			message, err := it.Next()
			if err == iterator.Done {
				p.logger.Info("iterator-is-done")
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

			var processErr error
			var retryable bool

			tag := p.subscription.ID()

			p.processTimer.Time(logger, func() {
				retryable, processErr = p.processor.Process(logger, message)
			}, tag)

			p.processCounter.Inc(logger, tag)

			if err != nil {
				logger.Error("failed-to-process-message", processErr)
			}

			message.Done(true)
		}
	}()

	<-signals

	p.logger.Info("told-to-exit")
	it.Stop()

	p.logger.Info("done")
	return nil
}

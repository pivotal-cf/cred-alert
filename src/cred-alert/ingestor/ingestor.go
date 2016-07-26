package ingestor

import (
	"cred-alert/metrics"
	"cred-alert/queue"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Ingestor

type Ingestor interface {
	IngestPushScan(lager.Logger, PushScan) error
}

type ingestor struct {
	whitelist *Whitelist
	taskQueue queue.Queue
	generator queue.UUIDGenerator

	requestCounter      metrics.Counter
	ignoredEventCounter metrics.Counter
}

func NewIngestor(
	taskQueue queue.Queue,
	emitter metrics.Emitter,
	whitelist *Whitelist,
	generator queue.UUIDGenerator,
) *ingestor {
	requestCounter := emitter.Counter("cred_alert.ingestor_requests")
	ignoredEventCounter := emitter.Counter("cred_alert.ignored_events")

	handler := &ingestor{
		taskQueue:           taskQueue,
		whitelist:           whitelist,
		generator:           generator,
		requestCounter:      requestCounter,
		ignoredEventCounter: ignoredEventCounter,
	}

	return handler
}

func (s *ingestor) IngestPushScan(logger lager.Logger, scan PushScan) error {
	logger = logger.Session("handle-event")

	if s.whitelist.IsIgnored(scan.Repository) {
		logger.Info("ignored-repo", lager.Data{
			"repo": scan.Repository,
		})

		s.ignoredEventCounter.Inc(logger)

		return nil
	}

	s.requestCounter.Inc(logger)

	id := s.generator.Generate()
	task := queue.PushEventPlan{
		Owner:      scan.Owner,
		Repository: scan.Repository,
		From:       scan.From,
		To:         scan.To,
	}.Task(id)

	err := s.taskQueue.Enqueue(task)
	if err != nil {
		logger.Error("failed-to-enqueue", err)
		return err
	}

	logger.Info("done")

	return nil
}

package ingestor

import (
	"cred-alert/metrics"
	"cred-alert/queue"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Ingestor

type Ingestor interface {
	IngestPushScan(lager.Logger, PushScan) error
}

type ingestor struct {
	enqueuer  queue.Enqueuer
	generator queue.UUIDGenerator

	requestCounter metrics.Counter
}

func NewIngestor(
	enqueuer queue.Enqueuer,
	emitter metrics.Emitter,
	metricPrefix string,
	generator queue.UUIDGenerator,
) Ingestor {
	requestCounter := emitter.Counter(metricPrefix + ".ingestor_requests")

	handler := &ingestor{
		enqueuer:       enqueuer,
		generator:      generator,
		requestCounter: requestCounter,
	}

	return handler
}

func (s *ingestor) IngestPushScan(logger lager.Logger, scan PushScan) error {
	logger = logger.Session("ingest-push-scan")
	logger.Debug("starting")

	s.requestCounter.Inc(logger)

	id := s.generator.Generate()

	task := queue.PushEventPlan{
		Owner:      scan.Owner,
		Repository: scan.Repository,
		PushTime:   scan.PushTime,
	}.Task(id)

	logger = logger.Session("enqueuing-task", lager.Data{
		"task-id": id,
	})

	err := s.enqueuer.Enqueue(task)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

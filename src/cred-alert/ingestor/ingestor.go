package ingestor

import (
	"cred-alert/metrics"
	"cred-alert/queue"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Ingestor

type Ingestor interface {
	IngestPushScan(lager.Logger, PushScan, string) error
}

type ingestor struct {
	taskQueue queue.Queue
	generator queue.UUIDGenerator

	requestCounter metrics.Counter
}

func NewIngestor(
	taskQueue queue.Queue,
	emitter metrics.Emitter,
	generator queue.UUIDGenerator,
) *ingestor {
	requestCounter := emitter.Counter("cred_alert.ingestor_requests")

	handler := &ingestor{
		taskQueue:      taskQueue,
		generator:      generator,
		requestCounter: requestCounter,
	}

	return handler
}

func (s *ingestor) IngestPushScan(logger lager.Logger, scan PushScan, githubID string) error {
	logger = logger.Session("ingest-push-scan")
	logger.Debug("starting")

	s.requestCounter.Inc(logger)

	id := s.generator.Generate()

	task := queue.PushEventPlan{
		Owner:      scan.Owner,
		Repository: scan.Repository,
		From:       scan.From,
		To:         scan.To,
		Private:    scan.Private,
	}.Task(id)

	logger = logger.Session("enqueuing-task", lager.Data{
		"task-id":   id,
		"github-id": githubID,
	})

	err := s.taskQueue.Enqueue(task)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

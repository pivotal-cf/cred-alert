package ingestor

import (
	"cred-alert/metrics"
	"cred-alert/queue"

	"github.com/pivotal-cf/paraphernalia/serve/requestid"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Ingestor

type Ingestor interface {
	IngestPushScan(lager.Logger, PushScan) error
}

type ingestor struct {
	enqueuer queue.Enqueuer

	requestCounter metrics.Counter
}

func NewIngestor(
	enqueuer queue.Enqueuer,
	emitter metrics.Emitter,
	metricPrefix string,
) Ingestor {
	requestCounter := emitter.Counter(metricPrefix + ".ingestor_requests")

	handler := &ingestor{
		enqueuer:       enqueuer,
		requestCounter: requestCounter,
	}

	return handler
}

func (s *ingestor) IngestPushScan(logger lager.Logger, scan PushScan) error {
	logger = logger.Session("ingest-push-scan")
	logger.Debug("starting")

	s.requestCounter.Inc(logger)

	task := queue.PushEventPlan{
		Owner:      scan.Owner,
		Repository: scan.Repository,
		PushTime:   scan.PushTime,
	}.Task(requestid.Generate())

	logger = logger.Session("enqueuing-task", lager.Data{
		"task-id": task.ID(),
	})

	err := s.enqueuer.Enqueue(task)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

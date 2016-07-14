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

//go:generate counterfeiter . UUIDGenerator

type UUIDGenerator interface {
	Generate() string
}

type ingestor struct {
	whitelist *Whitelist
	taskQueue queue.Queue
	generator UUIDGenerator

	requestCounter      metrics.Counter
	ignoredEventCounter metrics.Counter
}

func NewIngestor(taskQueue queue.Queue, emitter metrics.Emitter, whitelist *Whitelist, generator UUIDGenerator) *ingestor {
	requestCounter := emitter.Counter("cred_alert.webhook_hits")
	ignoredEventCounter := emitter.Counter("cred_alert.ignored_events")

	handler := &ingestor{
		taskQueue: taskQueue,
		whitelist: whitelist,
		generator: generator,

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

	for _, scanDiff := range scan.Diffs {
		id := s.generator.Generate()
		logger = logger.Session("enqueuing", lager.Data{
			"task-id": id,
		})

		logger.Debug("starting")

		task := queue.DiffScanPlan{
			Owner:      scan.Owner,
			Repository: scan.Repository,
			Ref:        scan.Ref,
			From:       scanDiff.From,
			To:         scanDiff.To,
		}.Task(id)

		err := s.taskQueue.Enqueue(task)
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		logger.Info("done")
	}

	return nil
}

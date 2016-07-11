package ingestor

import (
	"cred-alert/metrics"
	"cred-alert/queue"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Ingestor

type Ingestor interface {
	IngestPushScan(lager.Logger, PushScan)
}

type ingestor struct {
	foreman   queue.Foreman
	whitelist *Whitelist
	taskQueue queue.Queue

	requestCounter      metrics.Counter
	ignoredEventCounter metrics.Counter
}

func NewIngestor(taskQueue queue.Queue, emitter metrics.Emitter, whitelist *Whitelist) *ingestor {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	ignoredEventCounter := emitter.Counter("cred_alert.ignored_events")

	handler := &ingestor{
		taskQueue: taskQueue,
		whitelist: whitelist,

		requestCounter:      requestCounter,
		ignoredEventCounter: ignoredEventCounter,
	}

	return handler
}

func (s *ingestor) IngestPushScan(logger lager.Logger, scan PushScan) {
	logger = logger.Session("handle-event")

	if s.whitelist.IsIgnored(scan.Repository) {
		logger.Info("ignored-repo", lager.Data{
			"repo": scan.Repository,
		})

		s.ignoredEventCounter.Inc(logger)

		return
	}

	s.requestCounter.Inc(logger)

	for _, scanDiff := range scan.Diffs {
		task := queue.DiffScanPlan{
			Owner:      scan.Owner,
			Repository: scan.Repository,
			Start:      scanDiff.Start,
			End:        scanDiff.End,
		}.Task()

		err := s.taskQueue.Enqueue(task)
		if err != nil {
			logger.Error("enqueuing-failed", err)
		}
	}
}

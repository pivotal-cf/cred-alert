package webhook

import (
	"cred-alert/metrics"
	"cred-alert/queue"
	"fmt"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . EventHandler

type EventHandler interface {
	HandleEvent(lager.Logger, PushScan)
}

type eventHandler struct {
	foreman   *queue.Foreman
	whitelist *Whitelist

	requestCounter      metrics.Counter
	ignoredEventCounter metrics.Counter
}

func NewEventHandler(foreman *queue.Foreman, emitter metrics.Emitter, whitelist *Whitelist) *eventHandler {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	ignoredEventCounter := emitter.Counter("cred_alert.ignored_events")

	handler := &eventHandler{
		foreman:   foreman,
		whitelist: whitelist,

		requestCounter:      requestCounter,
		ignoredEventCounter: ignoredEventCounter,
	}

	return handler
}

func (s *eventHandler) HandleEvent(logger lager.Logger, scan PushScan) {
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

		job, err := s.foreman.BuildJob(queue.NoopAck(task))
		if err != nil {
			logger.Error("failed-building-job", err)
			return
		}

		job.Run(logger)
	}
}

type PushScan struct {
	Owner      string
	Repository string

	Diffs []PushScanDiff
}

func (p PushScan) FullRepoName() string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repository)
}

func (p PushScan) FirstCommit() string {
	return p.Diffs[0].Start
}

func (p PushScan) LastCommit() string {
	return p.Diffs[len(p.Diffs)-1].End
}

type PushScanDiff struct {
	Start string
	End   string
}

const initalCommitParentHash = "0000000000000000000000000000000000000000"

func Extract(event github.PushEvent) (PushScan, bool) {
	if event.Before == nil || *event.Before == initalCommitParentHash {
		return PushScan{}, false
	}

	if len(event.Commits) == 0 {
		return PushScan{}, false
	}

	diffs := []PushScanDiff{
		{Start: *event.Before, End: *event.Commits[0].ID},
	}

	for i, _ := range event.Commits {
		if i == len(event.Commits)-1 {
			break
		}

		start := *event.Commits[i].ID
		end := *event.Commits[i+1].ID

		diffs = append(diffs, PushScanDiff{
			Start: start,
			End:   end,
		})
	}

	return PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		Diffs:      diffs,
	}, true
}

package queue

import (
	"github.com/pivotal-golang/lager"

	"cred-alert/github"
	"cred-alert/metrics"
	"cred-alert/models"
)

type AncestryScanJob struct {
	AncestryScanPlan

	commitRepository     models.CommitRepository
	depthReachedCounter  metrics.Counter
	initialCommitCounter metrics.Counter
	client               github.Client
	taskQueue            Queue
}

func NewAncestryScanJob(plan AncestryScanPlan, commitRepository models.CommitRepository, client github.Client, emitter metrics.Emitter, taskQueue Queue) *AncestryScanJob {
	depthReachedCounter := emitter.Counter("cred_alert.max-depth-reached")
	initialCommitCounter := emitter.Counter("cred_alert.initial-commit-scanned")
	job := &AncestryScanJob{
		AncestryScanPlan: plan,

		commitRepository:     commitRepository,
		client:               client,
		depthReachedCounter:  depthReachedCounter,
		initialCommitCounter: initialCommitCounter,
		taskQueue:            taskQueue,
	}

	return job
}

func (j *AncestryScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("scanning-ancestry", lager.Data{
		"sha":              j.SHA,
		"org":              j.Owner,
		"repo":             j.Repository,
		"commit-timestamp": j.CommitTimestamp,
	})

	isRegistered, err := j.commitRepository.IsCommitRegistered(logger, j.SHA)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	if isRegistered {
		logger.Debug("known-commit")
		return nil
	}

	if j.Depth == 0 { // TODO: < 0
		if err := j.enqueueRefScan(); err != nil {
			logger.Error("failed", err)
			return err
		}

		if err = j.registerCommit(logger); err != nil {
			logger.Error("failed", err)
			return err
		}

		logger.Info("max-depth-reached")
		j.depthReachedCounter.Inc(logger)
		return nil
	}

	parents, err := j.client.Parents(j.Owner, j.Repository, j.SHA)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	logger.Debug("parents", lager.Data{"parents": parents})

	if len(parents) == 0 {
		if err := j.enqueueRefScan(); err != nil {
			logger.Error("failed", err)
			return err
		}
		logger.Info("reached-initial-commit")
		j.initialCommitCounter.Inc(logger)
	}

	for _, parent := range parents {
		if err := j.enqueueDiffScan(parent, j.SHA); err != nil {
			logger.Error("failed", err)
			return err
		}

		if err := j.enqueueAncestryScan(parent); err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	if err = j.registerCommit(logger); err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")

	return nil
}

func (j *AncestryScanJob) enqueueRefScan() error {
	task := RefScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		Ref:        j.SHA,
	}.Task("TODO")

	return j.taskQueue.Enqueue(task)
}

func (j *AncestryScanJob) enqueueAncestryScan(sha string) error {
	ancestryScan := AncestryScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        sha,
		Depth:      j.Depth - 1,
	}.Task("TODO")

	return j.taskQueue.Enqueue(ancestryScan)
}

func (j *AncestryScanJob) enqueueDiffScan(from string, to string) error {
	diffScan := DiffScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		From:       from,
		To:         to,
	}.Task("TODO")

	return j.taskQueue.Enqueue(diffScan)
}

func (j *AncestryScanJob) registerCommit(logger lager.Logger) error {
	return j.commitRepository.RegisterCommit(logger, &models.Commit{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        j.SHA,
	})
}

package queue

import (
	"github.com/pivotal-golang/lager"

	"cred-alert/db"
	"cred-alert/github"
	"cred-alert/metrics"
)

type AncestryScanJob struct {
	AncestryScanPlan

	commitRepository     db.CommitRepository
	depthReachedCounter  metrics.Counter
	initialCommitCounter metrics.Counter
	client               github.Client
	taskQueue            Queue
	id                   string
}

func NewAncestryScanJob(plan AncestryScanPlan, commitRepository db.CommitRepository, client github.Client, emitter metrics.Emitter, taskQueue Queue, id string) *AncestryScanJob {
	depthReachedCounter := emitter.Counter("cred_alert.max-depth-reached")
	initialCommitCounter := emitter.Counter("cred_alert.initial-commit-scanned")
	job := &AncestryScanJob{
		AncestryScanPlan: plan,

		commitRepository:     commitRepository,
		client:               client,
		depthReachedCounter:  depthReachedCounter,
		initialCommitCounter: initialCommitCounter,
		taskQueue:            taskQueue,
		id:                   id,
	}

	return job
}

func (j *AncestryScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("scanning-ancestry", lager.Data{
		"sha":     j.SHA,
		"owner":   j.Owner,
		"repo":    j.Repository,
		"task-id": j.id,
	})

	logger.Info("starting")

	isRegistered, err := j.commitRepository.IsCommitRegistered(logger, j.SHA)
	if err != nil {
		logger.Error("is-commit-registered-failed", err)
		return err
	}

	if isRegistered {
		logger.Info("known-commit")
		return nil
	}

	if j.Depth <= 0 {
		if err := j.enqueueRefScan(logger); err != nil {
			return err
		}

		if err = j.registerCommit(logger); err != nil {
			logger.Error("register-commit-failed", err)
			return err
		}

		logger.Info("max-depth-reached")
		j.depthReachedCounter.Inc(logger)
		return nil
	}

	parents, err := j.client.Parents(logger, j.Owner, j.Repository, j.SHA)
	if err != nil {
		logger.Error("fetching-parents-failed", err)
		return err
	}
	logger.Info("fetching-parents-succeeded", lager.Data{"parents": parents})

	if len(parents) == 0 {
		logger.Info("reached-initial-commit")
		if err := j.enqueueRefScan(logger); err != nil {
			return err
		}
		j.initialCommitCounter.Inc(logger)
	}

	for _, parent := range parents {
		if err := j.enqueueDiffScan(logger, parent, j.SHA); err != nil {
			return err
		}

		if err := j.enqueueAncestryScan(logger, parent); err != nil {
			return err
		}
	}

	if err = j.registerCommit(logger); err != nil {
		logger.Error("register-commit-failed", err)
		return err
	}

	logger.Info("done")

	return nil
}

func (j *AncestryScanJob) enqueueRefScan(logger lager.Logger) error {
	task := RefScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		Ref:        j.SHA,
		Private:    j.Private,
	}.Task(j.id)

	logger.Info("enqueuing-ref-scan")
	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Error("enqueuing-ref-scan-failed", err)
	} else {
		logger.Info("enqueuing-ref-scan-succeeded")
	}

	return err
}

func (j *AncestryScanJob) enqueueAncestryScan(logger lager.Logger, sha string) error {
	task := AncestryScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        sha,
		Depth:      j.Depth - 1,
		Private:    j.Private,
	}.Task(j.id)

	logger.Info("enqueuing-ancestry-scan")
	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Error("enqueuing-ancestry-scan-failed", err)
	} else {
		logger.Info("enqueuing-ancestry-scan-succeeded")
	}

	return err
}

func (j *AncestryScanJob) enqueueDiffScan(logger lager.Logger, from string, to string) error {
	task := DiffScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		From:       from,
		To:         to,
		Private:    j.Private,
	}.Task(j.id)

	logger.Info("enqueuing-diff-scan")
	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Error("enqueuing-diff-scan-failed", err)
	} else {
		logger.Info("enqueuing-diff-scan-succeeded")
	}

	return err
}

func (j *AncestryScanJob) registerCommit(logger lager.Logger) error {
	return j.commitRepository.RegisterCommit(logger, &db.Commit{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        j.SHA,
	})
}

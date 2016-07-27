package queue

import (
	"github.com/pivotal-golang/lager"

	"cred-alert/db"
	"cred-alert/githubclient"
	"cred-alert/metrics"
)

type AncestryScanJob struct {
	AncestryScanPlan

	commitRepository     db.CommitRepository
	depthReachedCounter  metrics.Counter
	initialCommitCounter metrics.Counter
	client               githubclient.Client
	taskQueue            Queue
	id                   string
}

func NewAncestryScanJob(plan AncestryScanPlan, commitRepository db.CommitRepository, client githubclient.Client, emitter metrics.Emitter, taskQueue Queue, id string) *AncestryScanJob {
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
		logger.Error("failed", err)
		return err
	}

	if isRegistered {
		logger.Info("known-commit")
		logger.Info("done")
		return nil
	}

	if j.Depth <= 0 {
		if err := j.enqueueRefScan(logger); err != nil {
			logger.Error("failed", err)
			return err
		}

		if err = j.registerCommit(logger); err != nil {
			logger.Error("failed", err)
			return err
		}

		logger.Info("max-depth-reached")
		j.depthReachedCounter.Inc(logger)
		logger.Info("done")
		return nil
	}

	info, err := j.client.CommitInfo(logger, j.Owner, j.Repository, j.SHA)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	parents := info.Parents
	logger.Info("fetching-parents-succeeded", lager.Data{"parents": parents})

	if len(parents) == 0 {
		logger.Info("reached-initial-commit")
		if err := j.enqueueRefScan(logger); err != nil {
			logger.Error("failed", err)
			return err
		}
		j.initialCommitCounter.Inc(logger)
	}

	for _, parent := range parents {
		if err := j.enqueueDiffScan(logger, parent, j.SHA); err != nil {
			logger.Error("failed", err)
			return err
		}

		if err := j.enqueueAncestryScan(logger, parent); err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	if err := j.enqueueCommitMessageScan(logger, j.SHA, info.Message); err != nil {
		logger.Error("failed", err)
		return err
	}

	if err = j.registerCommit(logger); err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *AncestryScanJob) enqueueRefScan(logger lager.Logger) error {
	logger = logger.Session("enqueue-ref-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"sha":        j.SHA,
		"private":    j.Private,
	})
	logger.Info("starting")

	task := RefScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		Ref:        j.SHA,
		Private:    j.Private,
	}.Task(j.id)

	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Session("enqueue").Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *AncestryScanJob) enqueueAncestryScan(logger lager.Logger, sha string) error {
	depth := (j.Depth - 1)

	logger = logger.Session("enqueue-ancestry-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"sha":        sha,
		"depth":      depth,
		"private":    j.Private,
	})
	logger.Info("starting")

	task := AncestryScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        sha,
		Depth:      depth,
		Private:    j.Private,
	}.Task(j.id)

	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Session("enqueue").Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *AncestryScanJob) enqueueDiffScan(logger lager.Logger, from string, to string) error {
	logger = logger.Session("enqueue-diff-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"from":       from,
		"to":         to,
		"private":    j.Private,
	})
	logger.Info("starting")

	task := DiffScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		From:       from,
		To:         to,
		Private:    j.Private,
	}.Task(j.id)

	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Session("enqueue").Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *AncestryScanJob) enqueueCommitMessageScan(logger lager.Logger, sha string, message string) error {
	logger = logger.Session("enqueue-commit-message-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"private":    j.Private,
		"sha":        sha,
	})
	logger.Info("starting")

	task := CommitMessageScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        sha,
		Private:    j.Private,
		Message:    message,
	}.Task(j.id)

	err := j.taskQueue.Enqueue(task)
	if err != nil {
		logger.Session("enqueue").Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *AncestryScanJob) registerCommit(logger lager.Logger) error {
	return j.commitRepository.RegisterCommit(logger, &db.Commit{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        j.SHA,
	})
}

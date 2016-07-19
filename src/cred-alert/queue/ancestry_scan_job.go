package queue

import (
	"github.com/pivotal-golang/lager"

	"cred-alert/github"
	"cred-alert/models"
)

type AncestryScanJob struct {
	AncestryScanPlan

	commitRepository models.CommitRepository
	client           github.Client
	taskQueue        Queue
}

func NewAncestryScanJob(plan AncestryScanPlan, commitRepository models.CommitRepository, client github.Client, taskQueue Queue) *AncestryScanJob {
	job := &AncestryScanJob{
		AncestryScanPlan: plan,

		commitRepository: commitRepository,
		client:           client,
		taskQueue:        taskQueue,
	}

	return job
}

func (j *AncestryScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("scanning-ancestry")

	isRegistered, err := j.commitRepository.IsCommitRegistered(logger, j.SHA)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	if isRegistered {
		return nil
	}

	// TODO: error checking + tests

	if j.Depth == 0 { // TODO: < 0
		task := RefScanPlan{
			Owner:      j.Owner,
			Repository: j.Repository,
			Ref:        j.SHA,
		}.Task("TODO")

		if err := j.taskQueue.Enqueue(task); err != nil {
			logger.Error("failed", err)
			return err
		}

		err = j.commitRepository.RegisterCommit(logger, &models.Commit{
			Owner:      j.Owner,
			Repository: j.Repository,
			SHA:        j.SHA,
		})
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		return nil
	}

	parents, err := j.client.Parents(j.Owner, j.Repository, j.SHA)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	for _, parent := range parents {
		diffScan := DiffScanPlan{
			Owner:      j.Owner,
			Repository: j.Repository,
			From:       parent,
			To:         j.SHA,
		}.Task("TODO")

		if err := j.taskQueue.Enqueue(diffScan); err != nil {
			logger.Error("failed", err)
			return err
		}

		ancestryScan := AncestryScanPlan{
			Owner:      j.Owner,
			Repository: j.Repository,
			SHA:        parent,
			Depth:      j.Depth - 1,
		}.Task("TODO")

		if err := j.taskQueue.Enqueue(ancestryScan); err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	err = j.commitRepository.RegisterCommit(logger, &models.Commit{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        j.SHA,
	})
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	return nil
}

package queue

import (
	"cred-alert/db"

	"code.cloudfoundry.org/lager"
)

type PushEventJob struct {
	PushEventPlan
	id               string
	taskQueue        Queue
	commitRepository db.CommitRepository
}

func NewPushEventJob(
	plan PushEventPlan,
	id string,
	taskQueue Queue,
	commitRepository db.CommitRepository,
) *PushEventJob {
	return &PushEventJob{
		PushEventPlan:    plan,
		id:               id,
		taskQueue:        taskQueue,
		commitRepository: commitRepository,
	}
}

func (j *PushEventJob) Run(logger lager.Logger) error {
	logger = logger.Session("push-event-job", lager.Data{
		"owner": j.Owner,
		"repo":  j.Repository,
		"from":  j.From,
		"to":    j.To,
	})

	registered, err := j.commitRepository.IsRepoRegistered(logger, j.Owner, j.Repository)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	if !registered {
		err = j.taskQueue.Enqueue(RefScanPlan{
			Owner:      j.Owner,
			Repository: j.Repository,
			Ref:        j.From,
		}.Task(j.id))
		if err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	return j.taskQueue.Enqueue(AncestryScanPlan{
		Owner:      j.Owner,
		Repository: j.Repository,
		SHA:        j.To,
		Depth:      DefaultScanDepth,
	}.Task(j.id))
}

package queue

import "github.com/pivotal-golang/lager"

type RefScanJob struct {
	RefScanPlan
}

func NewRefScanJob(plan RefScanPlan) *RefScanJob {
	job := &RefScanJob{
		RefScanPlan: plan,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	return nil
}

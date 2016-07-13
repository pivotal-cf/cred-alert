package queue

import (
	"cred-alert/github"

	"github.com/pivotal-golang/lager"
)

type RefScanJob struct {
	RefScanPlan

	client github.Client
}

func NewRefScanJob(plan RefScanPlan, client github.Client) *RefScanJob {
	job := &RefScanJob{
		RefScanPlan: plan,

		client: client,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	// Get a link to an archive
	j.client.ArchiveLink(logger, j.Owner, j.Repository)

	// Download that archive into a temporary directory

	// Unpack that archive into another temporary directory

	// Scan for credentials in the unpacked archive

	// Notify of any credentials

	// Clean up all files

	return nil
}

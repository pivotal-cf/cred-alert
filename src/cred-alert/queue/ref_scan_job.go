package queue

import (
	"net/url"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

type RefScanJob struct {
	RefScanPlan

	archiver Archiver
}

//go:generate counterfeiter . Archiver

type Archiver interface {
	GetArchiveLink(owner, repo, archiveFormat string, opt *github.RepositoryContentGetOptions) (*url.URL, *github.Response, error)
}

func NewRefScanJob(plan RefScanPlan, archiver Archiver) *RefScanJob {
	job := &RefScanJob{
		RefScanPlan: plan,

		archiver: archiver,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	// Get a link to an archive
	j.archiver.GetArchiveLink(j.Owner, j.Repository, "tarball", &github.RepositoryContentGetOptions{
		Ref: j.Ref,
	})

	// Download that archive into a temporary directory

	// Unpack that archive into another temporary directory

	// Scan for credentials in the unpacked archive

	// Notify of any credentials

	// Clean up all files

	return nil
}

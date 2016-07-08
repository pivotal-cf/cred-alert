package queue

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners/git"
	"cred-alert/sniff"
	"errors"

	gh "cred-alert/github"

	"github.com/pivotal-golang/lager"
)

type DiffScanJob struct {
	DiffScanPlan
	githubClient      gh.Client
	sniff             func(lager.Logger, sniff.Scanner, func(sniff.Line))
	credentialCounter metrics.Counter
	notifier          notifications.Notifier
}

func NewDiffScanJob(githubClient gh.Client, sniff func(lager.Logger, sniff.Scanner, func(sniff.Line)), emitter metrics.Emitter, notifier notifications.Notifier, plan DiffScanPlan) *DiffScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &DiffScanJob{
		DiffScanPlan: plan,
		githubClient: githubClient,
		sniff:        sniff,

		credentialCounter: credentialCounter,
		notifier:          notifier,
	}

	return job
}

func (j *DiffScanJob) Run(logger lager.Logger) error {
	diff, err := j.githubClient.CompareRefs(logger, j.Owner, j.Repository, j.Start, j.End)
	if err != nil {
		logger.Error("failed-fetch-diff", errors.New("Couldn't fetch diff "+j.Start+" "+j.End))
	}
	diffScanner := git.NewDiffScanner(diff)
	handleViolation := j.createHandleViolation(logger, j.End, j.Owner+"/"+j.Repository)
	j.sniff(logger, diffScanner, handleViolation)

	return nil
}

func (j *DiffScanJob) createHandleViolation(logger lager.Logger, sha string, repoName string) func(sniff.Line) {
	return func(line sniff.Line) {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})

		j.notifier.SendNotification(logger, repoName, sha, line)

		j.credentialCounter.Inc(logger)
	}
}

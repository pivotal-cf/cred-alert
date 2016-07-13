package queue

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/git"
	"cred-alert/sniff"

	gh "cred-alert/github"

	"github.com/pivotal-golang/lager"
)

type DiffScanJob struct {
	DiffScanPlan
	githubClient      gh.Client
	sniff             sniff.SniffFunc
	credentialCounter metrics.Counter
	notifier          notifications.Notifier
}

func NewDiffScanJob(githubClient gh.Client, sniff sniff.SniffFunc, emitter metrics.Emitter, notifier notifications.Notifier, plan DiffScanPlan) *DiffScanJob {
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
	logger = logger.Session("diff-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"from":       j.From,
		"to":         j.To,
	})

	diff, err := j.githubClient.CompareRefs(logger, j.Owner, j.Repository, j.From, j.To)
	if err != nil {
		return err
	}

	diffScanner := git.NewDiffScanner(diff)
	handleViolation := j.createHandleViolation(logger, j.To, j.Owner+"/"+j.Repository)

	j.sniff(logger, diffScanner, handleViolation)

	logger.Info("done")

	return nil
}

func (j *DiffScanJob) createHandleViolation(logger lager.Logger, sha string, repoName string) func(scanners.Line) {
	return func(line scanners.Line) {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})

		j.notifier.SendNotification(logger, repoName, sha, line)

		j.credentialCounter.Inc(logger)
	}
}

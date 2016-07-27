package queue

import (
	"cred-alert/db"
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

	diffScanRepository db.DiffScanRepository
	githubClient       gh.Client
	sniffer            sniff.Sniffer
	credentialCounter  metrics.Counter
	notifier           notifications.Notifier
	id                 string
}

func NewDiffScanJob(githubClient gh.Client, sniffer sniff.Sniffer, emitter metrics.Emitter, notifier notifications.Notifier, diffScanRepository db.DiffScanRepository, plan DiffScanPlan, id string) *DiffScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &DiffScanJob{
		DiffScanPlan:       plan,
		diffScanRepository: diffScanRepository,
		githubClient:       githubClient,
		sniffer:            sniffer,

		credentialCounter: credentialCounter,
		notifier:          notifier,
		id:                id,
	}

	return job
}

func (j *DiffScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("diff-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"from":       j.From,
		"to":         j.To,
		"task-id":    j.id,
		"private":    j.Private,
	})
	logger.Info("starting")

	diff, err := j.githubClient.CompareRefs(logger, j.Owner, j.Repository, j.From, j.To)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	diffScanner := git.NewDiffScanner(diff)
	credentialsFound := false
	handleViolation := j.createHandleViolation(logger, j.To, j.Owner+"/"+j.Repository, &credentialsFound)

	err = j.sniffer.Sniff(logger, diffScanner, handleViolation)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = j.diffScanRepository.SaveDiffScan(logger, &db.DiffScan{
		Owner:           j.Owner,
		Repository:      j.Repository,
		FromCommit:      j.From,
		ToCommit:        j.To,
		CredentialFound: credentialsFound,
	})
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (j *DiffScanJob) createHandleViolation(logger lager.Logger, sha string, repoName string, credentialsFound *bool) func(scanners.Line) error {
	return func(line scanners.Line) error {
		logger.Info("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})

		err := j.notifier.SendNotification(logger, repoName, sha, line, j.Private)
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		*credentialsFound = true
		tag := "public"
		if j.Private {
			tag = "private"
		}
		j.credentialCounter.Inc(logger, tag)

		logger.Info("done")
		return nil
	}
}

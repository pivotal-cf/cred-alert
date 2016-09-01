package queue

import (
	"cred-alert/db"
	"cred-alert/githubclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
)

type DiffScanJob struct {
	DiffScanPlan

	diffScanRepository db.DiffScanRepository
	scanRepository     db.ScanRepository
	githubClient       githubclient.Client
	sniffer            sniff.Sniffer
	credentialCounter  metrics.Counter
	notifier           notifications.Notifier
	id                 string
}

func NewDiffScanJob(
	githubClient githubclient.Client,
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	diffScanRepository db.DiffScanRepository,
	scanRepository db.ScanRepository,
	plan DiffScanPlan,
	id string,
) *DiffScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &DiffScanJob{
		DiffScanPlan:       plan,
		diffScanRepository: diffScanRepository,
		scanRepository:     scanRepository,
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
	logger.Debug("starting")

	scan := j.scanRepository.Start(logger, "diff-scan")

	diff, err := j.githubClient.CompareRefs(logger, j.Owner, j.Repository, j.From, j.To)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	scanner := diffscanner.NewDiffScanner(diff)
	alerts := []notifications.Notification{}

	handleViolation := j.createHandleViolation(j.To, j.Owner+"/"+j.Repository, scan, &alerts)

	err = j.sniffer.Sniff(logger, scanner, handleViolation)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = j.notifier.SendBatchNotification(logger, alerts)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = scan.Finish()
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = j.diffScanRepository.SaveDiffScan(logger, &db.DiffScan{
		Owner:           j.Owner,
		Repository:      j.Repository,
		FromCommit:      j.From,
		ToCommit:        j.To,
		CredentialFound: len(alerts) > 0,
	})
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

func (j *DiffScanJob) createHandleViolation(
	sha string,
	repoName string,
	scan db.ActiveScan,
	alerts *[]notifications.Notification,
) func(lager.Logger, scanners.Line) error {
	return func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})
		logger.Debug("starting")

		credential := db.Credential{
			Owner:      j.Owner,
			Repository: j.Repository,
			SHA:        sha,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		}

		scan.RecordCredential(credential)

		*alerts = append(*alerts, notifications.Notification{
			Owner:      j.Owner,
			Repository: j.Repository,
			Private:    j.Private,
			SHA:        sha,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		})

		tag := "public"
		if j.Private {
			tag = "private"
		}
		j.credentialCounter.Inc(logger, tag)

		logger.Debug("done")
		return nil
	}
}

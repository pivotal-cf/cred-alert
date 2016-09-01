package queue

import (
	"cred-alert/db"
	"cred-alert/githubclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"
	"io"

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

	alerts, err := j.scanDiffForCredentials(logger, scan, diff)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = j.reportCredentials(logger, alerts)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = scan.Finish()
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

func (j *DiffScanJob) scanDiffForCredentials(logger lager.Logger, scan db.ActiveScan, diff io.Reader) ([]notifications.Notification, error) {
	scanner := diffscanner.NewDiffScanner(diff)
	alerts := []notifications.Notification{}

	err := j.sniffer.Sniff(logger, scanner, func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         j.To,
		})
		logger.Debug("starting")

		scan.RecordCredential(db.Credential{
			Owner:      j.Owner,
			Repository: j.Repository,
			SHA:        j.To,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		})

		alerts = append(alerts, notifications.Notification{
			Owner:      j.Owner,
			Repository: j.Repository,
			Private:    j.Private,
			SHA:        j.To,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		})

		logger.Debug("done")

		return nil
	})
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	return alerts, nil
}

func (j *DiffScanJob) reportCredentials(logger lager.Logger, alerts []notifications.Notification) error {
	tag := "public"
	if j.Private {
		tag = "private"
	}
	j.credentialCounter.IncN(logger, len(alerts), tag)

	err := j.notifier.SendBatchNotification(logger, alerts)
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

	return nil
}

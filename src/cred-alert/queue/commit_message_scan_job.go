package queue

import (
	"cred-alert/db"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/textscanner"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
)

type CommitMessageJob struct {
	CommitMessageScanPlan

	sniffer           sniff.Sniffer
	credentialCounter metrics.Counter
	notifier          notifications.Notifier
	scanRepository    db.ScanRepository
	id                string
}

func NewCommitMessageJob(
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	scanRepository db.ScanRepository,
	plan CommitMessageScanPlan,
) *CommitMessageJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	return &CommitMessageJob{
		CommitMessageScanPlan: plan,
		sniffer:               sniffer,
		credentialCounter:     credentialCounter,
		notifier:              notifier,
		scanRepository:        scanRepository,
	}
}

func (j *CommitMessageJob) Run(logger lager.Logger) error {
	logger = logger.Session("scan-commit-message", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"private":    j.Private,
		"sha":        j.SHA,
	})

	logger.Debug("starting")

	scan := j.scanRepository.Start(logger, "commit-message-scan", nil, nil)

	textScanner := textscanner.New(j.Message)

	err := j.sniffer.Sniff(logger, textScanner, j.createHandleViolation(scan))
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

func (j *CommitMessageJob) createHandleViolation(scan db.ActiveScan) func(lager.Logger, scanners.Violation) error {
	return func(logger lager.Logger, violation scanners.Violation) error {
		logger = logger.Session("handle-violation")
		logger.Debug("starting")

		privacyTag := "public"
		if j.Private {
			privacyTag = "private"
		}

		j.credentialCounter.Inc(logger, privacyTag, "commit-message")

		credential := db.Credential{
			Owner:      j.Owner,
			Repository: j.Repository,
			SHA:        j.SHA,
			Path:       violation.Line.Path,
			LineNumber: violation.Line.LineNumber,
		}

		scan.RecordCredential(credential)

		notification := notifications.Notification{
			Owner:      j.Owner,
			Repository: j.Repository,
			Private:    j.Private,
			SHA:        j.SHA,
			Path:       violation.Line.Path,
			LineNumber: violation.Line.LineNumber,
		}

		if err := j.notifier.SendNotification(logger, notification); err != nil {
			return err
		}

		logger.Debug("done")
		return nil
	}
}

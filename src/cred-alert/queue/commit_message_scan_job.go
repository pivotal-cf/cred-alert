package queue

import (
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
	id                string
}

func NewCommitMessageJob(
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	plan CommitMessageScanPlan,
	id string,
) *CommitMessageJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	return &CommitMessageJob{
		CommitMessageScanPlan: plan,
		sniffer:               sniffer,
		credentialCounter:     credentialCounter,
		notifier:              notifier,
		id:                    id,
	}
}

func (j *CommitMessageJob) Run(logger lager.Logger) error {
	logger = logger.Session("scan-commit-message", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"private":    j.Private,
		"sha":        j.SHA,
		"task-id":    j.id,
	})

	logger.Info("starting")

	textScanner := textscanner.New(j.Message)

	err := j.sniffer.Sniff(logger, textScanner, j.createHandleViolation(logger))
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Info("done")

	return nil
}

func (j *CommitMessageJob) createHandleViolation(logger lager.Logger) func(scanners.Line) error {
	return func(line scanners.Line) error {
		logger.Info("found-credentials")

		privacyTag := "public"
		if j.Private {
			privacyTag = "private"
		}

		j.credentialCounter.Inc(logger, privacyTag, "commit-message")

		if err := j.notifier.SendNotification(logger, j.Owner, j.SHA, line, j.Private); err != nil {
			return err
		}

		return nil
	}
}

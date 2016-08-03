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
) *CommitMessageJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	return &CommitMessageJob{
		CommitMessageScanPlan: plan,
		sniffer:               sniffer,
		credentialCounter:     credentialCounter,
		notifier:              notifier,
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

	textScanner := textscanner.New(j.Message)

	err := j.sniffer.Sniff(logger, textScanner, j.createHandleViolation())
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")

	return nil
}

func (j *CommitMessageJob) createHandleViolation() func(lager.Logger, scanners.Line) error {
	return func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation")
		logger.Debug("starting")

		privacyTag := "public"
		if j.Private {
			privacyTag = "private"
		}

		j.credentialCounter.Inc(logger, privacyTag, "commit-message")

		if err := j.notifier.SendNotification(logger, j.Owner, j.SHA, line, j.Private); err != nil {
			return err
		}

		logger.Debug("done")
		return nil
	}
}

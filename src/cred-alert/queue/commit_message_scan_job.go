package queue

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/sniff"

	"github.com/pivotal-golang/lager"
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
	return nil
}

package queue

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/sniff"
	"encoding/json"
	"fmt"

	gh "cred-alert/github"

	"github.com/pivotal-golang/lager"
)

type Job interface {
	Run(lager.Logger) error
}

type Foreman struct {
	githubClient gh.Client
	sniff        func(lager.Logger, sniff.Scanner, func(sniff.Line))
	emitter      metrics.Emitter
	notifier     notifications.Notifier
}

func NewForeman(githubClient gh.Client, sniff func(lager.Logger, sniff.Scanner, func(sniff.Line)), emitter metrics.Emitter, notifier notifications.Notifier) *Foreman {
	foreman := &Foreman{
		githubClient: githubClient,
		sniff:        sniff,
		emitter:      emitter,
		notifier:     notifier,
	}

	return foreman
}

func (f *Foreman) BuildJob(task AckTask) (Job, error) {
	switch task.Type() {
	case "diff-scan":
		return f.buildDiffScan(task.Payload())
	default:
		return nil, fmt.Errorf("unknown task type: %s", task.Type())
	}
}

func (f *Foreman) buildDiffScan(payload string) (*DiffScanJob, error) {
	var diffScanPlan DiffScanPlan

	if err := json.Unmarshal([]byte(payload), &diffScanPlan); err != nil {
		return nil, err
	}

	return NewDiffScanJob(
		f.githubClient,
		f.sniff,
		f.emitter,
		f.notifier,
		diffScanPlan,
	), nil
}

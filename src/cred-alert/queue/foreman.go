package queue

import (
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"encoding/json"
	"fmt"

	gh "cred-alert/github"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Job

type Job interface {
	Run(lager.Logger) error
}

//go:generate counterfeiter . Foreman

type Foreman interface {
	BuildJob(Task) (Job, error)
}

type foreman struct {
	githubClient gh.Client
	sniffer      sniff.SniffFunc
	emitter      metrics.Emitter
	notifier     notifications.Notifier
}

func NewForeman(githubClient gh.Client, sniffer func(lager.Logger, sniff.Scanner, func(scanners.Line) error) error, emitter metrics.Emitter, notifier notifications.Notifier) *foreman {
	foreman := &foreman{
		githubClient: githubClient,
		sniffer:      sniffer,
		emitter:      emitter,
		notifier:     notifier,
	}

	return foreman
}

func (f *foreman) BuildJob(task Task) (Job, error) {
	switch task.Type() {
	case "diff-scan":
		return f.buildDiffScan(task.Payload())
	case "ref-scan":
		return f.buildRefScan(task.Payload())
	default:
		return nil, fmt.Errorf("unknown task type: %s", task.Type())
	}
}

func (f *foreman) buildDiffScan(payload string) (*DiffScanJob, error) {
	var diffScanPlan DiffScanPlan

	if err := json.Unmarshal([]byte(payload), &diffScanPlan); err != nil {
		return nil, err
	}

	return NewDiffScanJob(
		f.githubClient,
		f.sniffer,
		f.emitter,
		f.notifier,
		diffScanPlan,
	), nil
}

func (f *foreman) buildRefScan(payload string) (*RefScanJob, error) {
	var refScanPlan RefScanPlan

	if err := json.Unmarshal([]byte(payload), &refScanPlan); err != nil {
		return nil, err
	}

	return NewRefScanJob(
		refScanPlan,
		f.githubClient,
		f.sniffer,
		f.notifier,
		f.emitter,
	), nil
}

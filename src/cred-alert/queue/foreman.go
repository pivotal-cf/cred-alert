package queue

import (
	"cred-alert/db"
	"cred-alert/metrics"
	"cred-alert/mimetype"
	"cred-alert/notifications"
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
	githubClient       gh.Client
	sniffer            sniff.Sniffer
	emitter            metrics.Emitter
	notifier           notifications.Notifier
	diffScanRepository db.DiffScanRepository
	commitRepository   db.CommitRepository
	taskQueue          Queue
	mimetype           mimetype.Mimetype
}

func NewForeman(
	githubClient gh.Client,
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	diffScanRepository db.DiffScanRepository,
	commitRepository db.CommitRepository,
	taskQueue Queue,
	mimetype mimetype.Mimetype,
) *foreman {
	foreman := &foreman{
		githubClient:       githubClient,
		sniffer:            sniffer,
		emitter:            emitter,
		notifier:           notifier,
		diffScanRepository: diffScanRepository,
		commitRepository:   commitRepository,
		taskQueue:          taskQueue,
		mimetype:           mimetype,
	}

	return foreman
}

func (f *foreman) BuildJob(task Task) (Job, error) {
	switch task.Type() {
	case TaskTypeDiffScan:
		return f.buildDiffScan(task.ID(), task.Payload())
	case TaskTypeRefScan:
		return f.buildRefScan(task.ID(), task.Payload())
	case TaskTypeAncestryScan:
		return f.buildAncestryScan(task.ID(), task.Payload())
	case TaskTypePushEvent:
		return f.buildPushEventJob(task.ID(), task.Payload())
	default:
		return nil, fmt.Errorf("unknown task type: %s", task.Type())
	}
}

func (f *foreman) buildPushEventJob(id, payload string) (*PushEventJob, error) {
	var plan PushEventPlan

	if err := json.Unmarshal([]byte(payload), &plan); err != nil {
		return nil, err
	}

	return NewPushEventJob(
		plan,
		id,
		f.taskQueue,
		f.commitRepository,
	), nil
}

func (f *foreman) buildDiffScan(id, payload string) (*DiffScanJob, error) {
	var diffScanPlan DiffScanPlan

	if err := json.Unmarshal([]byte(payload), &diffScanPlan); err != nil {
		return nil, err
	}

	return NewDiffScanJob(
		f.githubClient,
		f.sniffer,
		f.emitter,
		f.notifier,
		f.diffScanRepository,
		diffScanPlan,
		id,
	), nil
}

func (f *foreman) buildRefScan(id, payload string) (*RefScanJob, error) {
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
		f.mimetype,
		id,
	), nil
}

func (f *foreman) buildAncestryScan(id, payload string) (*AncestryScanJob, error) {
	var ancestryScanPlan AncestryScanPlan

	if err := json.Unmarshal([]byte(payload), &ancestryScanPlan); err != nil {
		return nil, err
	}

	return NewAncestryScanJob(
		ancestryScanPlan,
		f.commitRepository,
		f.githubClient,
		f.emitter,
		f.taskQueue,
		id,
	), nil
}

package queue

import (
	"cred-alert/db"
	"cred-alert/githubclient"
	"cred-alert/inflator"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/sniff"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
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
	client             githubclient.Client
	sniffer            sniff.Sniffer
	emitter            metrics.Emitter
	notifier           notifications.Notifier
	diffScanRepository db.DiffScanRepository
	commitRepository   db.CommitRepository
	taskQueue          Queue
	expander           inflator.Inflator
	scratchSpace       inflator.ScratchSpace
}

func NewForeman(
	client githubclient.Client,
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	diffScanRepository db.DiffScanRepository,
	commitRepository db.CommitRepository,
	taskQueue Queue,
	expander inflator.Inflator,
	scratchSpace inflator.ScratchSpace,
) *foreman {
	foreman := &foreman{
		client:             client,
		sniffer:            sniffer,
		emitter:            emitter,
		notifier:           notifier,
		diffScanRepository: diffScanRepository,
		commitRepository:   commitRepository,
		taskQueue:          taskQueue,
		expander:           expander,
		scratchSpace:       scratchSpace,
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
	case TaskTypeCommitMessageScan:
		return f.buildCommitMessageJob(task.Payload())
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

func (f *foreman) buildCommitMessageJob(payload string) (*CommitMessageJob, error) {
	var plan CommitMessageScanPlan

	if err := json.Unmarshal([]byte(payload), &plan); err != nil {
		return nil, err
	}

	return NewCommitMessageJob(
		f.sniffer,
		f.emitter,
		f.notifier,
		plan,
	), nil
}

func (f *foreman) buildDiffScan(id, payload string) (*DiffScanJob, error) {
	var diffScanPlan DiffScanPlan

	if err := json.Unmarshal([]byte(payload), &diffScanPlan); err != nil {
		return nil, err
	}

	return NewDiffScanJob(
		f.client,
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
		f.client,
		f.sniffer,
		f.notifier,
		f.emitter,
		f.expander,
		f.scratchSpace,
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
		f.client,
		f.emitter,
		f.taskQueue,
		id,
	), nil
}

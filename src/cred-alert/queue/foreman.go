package queue

import (
	"encoding/json"
	"fmt"
)

type Job interface {
	Run() error
}

type Foreman struct{}

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

	err := json.Unmarshal([]byte(payload), &diffScanPlan)
	if err != nil {
		panic("TODO: test me")
	}

	return &DiffScanJob{
		DiffScanPlan: diffScanPlan,
	}, nil
}

type DiffScanJob struct {
	DiffScanPlan
}

func (t *DiffScanJob) Run() error {
	return nil
}

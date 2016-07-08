package queue

import "encoding/json"

type DiffScanPlan struct {
	Owner      string
	Repository string

	Start string
	End   string
}

func (p DiffScanPlan) Task() Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		typee:   "diff-scan",
		payload: string(payload),
	}
}

type basicTask struct {
	typee   string
	payload string
}

func (t basicTask) Type() string {
	return t.typee
}

func (t basicTask) Payload() string {
	return t.payload
}

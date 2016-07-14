package queue

import "encoding/json"

type DiffScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`

	From string `json:"from"`
	To   string `json:"to"`
}

func (p DiffScanPlan) Task(id string) Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		id:      id,
		typee:   "diff-scan",
		payload: string(payload),
	}
}

type RefScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`

	Ref string `json:"ref"`
}

func (p RefScanPlan) Task(id string) Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		id:      id,
		typee:   "ref-scan",
		payload: string(payload),
	}
}

type basicTask struct {
	id      string
	typee   string
	payload string
}

func (t basicTask) ID() string {
	return t.id
}

func (t basicTask) Type() string {
	return t.typee
}

func (t basicTask) Payload() string {
	return t.payload
}

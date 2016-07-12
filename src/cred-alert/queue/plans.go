package queue

import "encoding/json"

type DiffScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`

	From string `json:"from"`
	To   string `json:"to"`
}

func (p DiffScanPlan) Task() Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		typee:   "diff-scan",
		payload: string(payload),
	}
}

type RefScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`

	Ref string `json:"ref"`
}

func (p RefScanPlan) Task() Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		typee:   "ref-scan",
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

package queue

import "encoding/json"

const DefaultScanDepth = 10

type DiffScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`

	From string `json:"from"`
	To   string `json:"to"`
}

func (p DiffScanPlan) Task(id string) Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		id:      id,
		typee:   TaskTypeDiffScan,
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
		typee:   TaskTypeRefScan,
		payload: string(payload),
	}
}

type AncestryScanPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	SHA        string `json:"sha"`
	Depth      int    `json:"depth"`
}

func (a AncestryScanPlan) Task(id string) Task {
	payload, _ := json.Marshal(a)

	return basicTask{
		id:      id,
		typee:   TaskTypeAncestryScan,
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

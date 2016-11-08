package queue

import "encoding/json"

const TaskTypePushEvent = "push-event"

type PushEventPlan struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	From       string `json:"from"`
	To         string `json:"to"`
	Private    bool   `json:"private"`
}

func (p PushEventPlan) Task(id string) Task {
	payload, _ := json.Marshal(p)

	return basicTask{
		id:      id,
		typee:   TaskTypePushEvent,
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

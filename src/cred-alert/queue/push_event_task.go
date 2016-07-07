package queue

import (
	"errors"

	"github.com/google/go-github/github"
)

type pushEventTask struct {
	data map[string]interface{}
}

func NewPushEventTask(event github.PushEvent) *pushEventTask {
	task := &pushEventTask{}

	task.data = make(map[string]interface{})
	task.data["event"] = event

	return task
}

func (t *pushEventTask) Data() map[string]interface{} {
	return t.data
}

func (t *pushEventTask) Receipt() string {
	return "TODO add receipt"
}

func GetEvent(task Task) (github.PushEvent, error) {
	if event, ok := task.Data()["event"]; !ok {
		return github.PushEvent{}, errors.New("No event in this task")
	} else {
		if pushEvent, ok := event.(github.PushEvent); !ok {
			return github.PushEvent{}, errors.New("Value is not a push event")
		} else {
			return pushEvent, nil
		}
	}
}

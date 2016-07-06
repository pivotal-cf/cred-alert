package queue

import "github.com/pivotal-golang/lager"

type localQueue struct {
	logger lager.Logger
	tasks  []Task
}

func NewLocalQueue(logger lager.Logger) *localQueue {
	return &localQueue{}
}

func (q *localQueue) Enqueue(t Task) error {
	q.tasks = append(q.tasks, t)
	return nil
}

func (q *localQueue) Dequeue() (Task, error) {
	if len(q.tasks) <= 0 {
		return nil, EmptyQueueError{}
	}

	var dequeued Task
	dequeued, q.tasks = q.tasks[0], q.tasks[1:len(q.tasks)]

	return dequeued, nil
}

func (q *localQueue) Remove(Task) error {
	return nil
}

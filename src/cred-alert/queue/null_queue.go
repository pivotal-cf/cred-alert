package queue

import "github.com/pivotal-golang/lager"

type nullQueue struct {
	logger lager.Logger
}

func NewNullQueue(logger lager.Logger) *nullQueue {
	return &nullQueue{
		logger: logger,
	}
}

func (q *nullQueue) Enqueue(task Task) error {
	q.logger.Info("enqueue-task")
	return nil
}

func (q *nullQueue) Dequeue() (Task, error) {
	q.logger.Info("dequeue-task")
	return nil, nil
}

func (q *nullQueue) Remove(task Task) error {
	q.logger.Info("remove-task")
	return nil
}

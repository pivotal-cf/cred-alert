package queue

//go:generate counterfeiter . Queue

type Queue interface {
	Enqueue(Task) error
	Dequeue() (AckTask, error)

	EnqueuePlan(Plan) error
}

//go:generate counterfeiter . Task

type Task interface {
	Type() string
	Payload() string
}

//go:generate counterfeiter . AckTask

type AckTask interface {
	Task

	Ack() error
}

type noopAcker struct {
	Task
}

func (n *noopAcker) Ack() error {
	return nil
}

func NoopAck(task Task) AckTask {
	return &noopAcker{Task: task}
}

//go:generate counterfeiter . Plan

type Plan interface {
	Task() Task
}

type EmptyQueueError struct {
	error
}

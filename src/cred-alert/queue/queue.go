package queue

//go:generate counterfeiter . Queue

type Queue interface {
	Enqueue(Task) error
	Dequeue() (AckTask, error)
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

//go:generate counterfeiter . Plan

type Plan interface {
	Task() Task
}

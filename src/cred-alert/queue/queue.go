package queue

//go:generate counterfeiter . Enqueuer

type Enqueuer interface {
	Enqueue(Task) error
}

//go:generate counterfeiter . Dequeuer

type Dequeuer interface {
	Dequeue() (AckTask, error)
}

//go:generate counterfeiter . Queue

type Queue interface {
	Enqueuer
	Dequeuer
}

//go:generate counterfeiter . Task

type Task interface {
	ID() string
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
	Task(string) Task
}

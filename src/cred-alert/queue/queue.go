package queue

//go:generate counterfeiter . Enqueuer

type Enqueuer interface {
	Enqueue(Task) error
}

//go:generate counterfeiter . Task

type Task interface {
	ID() string
	Type() string
	Payload() string
}

//go:generate counterfeiter . Plan

type Plan interface {
	Task(string) Task
}

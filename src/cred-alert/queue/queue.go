package queue

//go:generate counterfeiter . Queue

type Queue interface {
	Enqueue(Task) error
	Dequeue() (Task, error)
	Remove(Task) error
}

//go:generate counterfeiter . Task

type Task interface {
	Data() map[string]interface{}
	Receipt() string
}

type EmptyQueueError struct {
	error
}

package worker

import (
	"os"

	"github.com/pivotal-golang/lager"

	"cred-alert/queue"
)

type worker struct {
	logger  lager.Logger
	foreman queue.Foreman
	queue   queue.Queue
}

func New(logger lager.Logger, foreman queue.Foreman, queue queue.Queue) *worker {
	return &worker{
		logger:  logger.Session("worker"),
		foreman: foreman,
		queue:   queue,
	}
}

func (w *worker) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	for {
		tasks, errs := w.dequeue()

		select {
		case task := <-tasks:
			w.processTask(w.logger, task)
		case err := <-errs:
			w.logger.Error("failed-to-dequeue", err)
		case <-signals:
			return nil
		}
	}
}

func (w *worker) dequeue() (<-chan queue.AckTask, <-chan error) {
	taskChan := make(chan queue.AckTask, 1)
	errChan := make(chan error, 1)

	go func(tChan chan queue.AckTask, eChan chan error) {
		task, err := w.queue.Dequeue()
		if err != nil {
			eChan <- err
			return
		}

		tChan <- task
	}(taskChan, errChan)

	return taskChan, errChan
}

func (w *worker) processTask(logger lager.Logger, task queue.AckTask) {
	job, err := w.foreman.BuildJob(task)
	if err != nil {
		logger.Error("building-job-failed", err)
		return
	}

	err = job.Run(logger)
	if err != nil {
		logger.Error("running-job-failed", err)
		return
	}

	err = task.Ack()
	if err != nil {
		logger.Error("acking-task-failed", err)
		return
	}
}

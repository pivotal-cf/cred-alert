package worker

import (
	"os"

	"github.com/pivotal-golang/lager"

	"cred-alert/metrics"
	"cred-alert/queue"
)

type worker struct {
	logger  lager.Logger
	foreman queue.Foreman
	queue   queue.Queue

	failedJobs metrics.Counter
	failedAcks metrics.Counter
}

func New(logger lager.Logger, foreman queue.Foreman, queue queue.Queue, emitter metrics.Emitter) *worker {
	failedJobs := emitter.Counter("cred_alert.failed_jobs")
	failedAcks := emitter.Counter("cred_alert.failed_acks")

	return &worker{
		logger:  logger.Session("worker"),
		foreman: foreman,
		queue:   queue,

		failedJobs: failedJobs,
		failedAcks: failedAcks,
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
		w.failedJobs.Inc(logger)
		return
	}

	err = job.Run(logger)
	if err != nil {
		logger.Error("running-job-failed", err)
		w.failedJobs.Inc(logger)
		return
	}

	err = task.Ack()
	if err != nil {
		logger.Error("acking-task-failed", err)
		w.failedAcks.Inc(logger)
		return
	}
}

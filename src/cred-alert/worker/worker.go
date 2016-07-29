package worker

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"

	"cred-alert/metrics"
	"cred-alert/queue"
)

type worker struct {
	logger  lager.Logger
	foreman queue.Foreman
	queue   queue.Queue

	failedJobs metrics.Counter
	failedAcks metrics.Counter
	taskTimer  metrics.Timer
}

func New(logger lager.Logger, foreman queue.Foreman, queue queue.Queue, emitter metrics.Emitter) *worker {
	failedJobs := emitter.Counter("cred_alert.failed_jobs")
	failedAcks := emitter.Counter("cred_alert.failed_acks")
	taskTimer := emitter.Timer("cred_alert.task_duration")

	return &worker{
		logger:  logger.Session("worker"),
		foreman: foreman,
		queue:   queue,

		failedJobs: failedJobs,
		failedAcks: failedAcks,
		taskTimer:  taskTimer,
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
	logger = logger.Session("process-task", lager.Data{
		"task-id":   task.ID(),
		"task-type": task.Type(),
	})
	logger.Info("starting")

	job, err := w.foreman.BuildJob(task)
	if err != nil {
		logger.Session("build-job").Error("failed", err)
		w.failedJobs.Inc(logger)
		return
	}

	w.taskTimer.Time(logger, func() {
		err = job.Run(logger)
	}, fmt.Sprintf("tasktype:%s", task.Type()))

	if err != nil {
		logger.Error("failed", err)
		w.failedJobs.Inc(logger)
		return
	}

	err = task.Ack()
	if err != nil {
		logger.Session("ack-task").Error("failed", err)
		w.failedAcks.Inc(logger)
		return
	}

	logger.Debug("done")
}

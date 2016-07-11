package worker_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue/queuefakes"
	"cred-alert/worker"
)

var _ = Describe("Worker", func() {
	var (
		logger  *lagertest.TestLogger
		foreman *queuefakes.FakeForeman
		queue   *queuefakes.FakeQueue
		emitter *metricsfakes.FakeEmitter

		failedJobs *metricsfakes.FakeCounter
		failedAcks *metricsfakes.FakeCounter
		fakeTimer  *metricsfakes.FakeTimer

		process ifrit.Process
		job     *queuefakes.FakeJob
		task    *queuefakes.FakeAckTask
		runner  ifrit.Runner
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("worker")

		failedJobs = &metricsfakes.FakeCounter{}
		failedAcks = &metricsfakes.FakeCounter{}
		fakeTimer = &metricsfakes.FakeTimer{}
		fakeTimer.TimeStub = func(logger lager.Logger, fn func(), tags ...string) {
			fn()
		}

		emitter = &metricsfakes.FakeEmitter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.failed_jobs":
				return failedJobs
			case "cred_alert.failed_acks":
				return failedAcks
			default:
				panic("unexpected counter name! " + name)
			}
		}
		emitter.TimerStub = func(name string) metrics.Timer {
			switch name {
			case "cred_alert.task_duration":
				return fakeTimer
			default:
				panic("unexpected timer name! " + name)
			}
		}

		foreman = &queuefakes.FakeForeman{}
		job = &queuefakes.FakeJob{}
		job.RunReturns(nil)
		task = &queuefakes.FakeAckTask{}
		foreman.BuildJobReturns(job, nil)

		queue = &queuefakes.FakeQueue{}
	})

	JustBeforeEach(func() {
		runner = worker.New(logger, foreman, queue, emitter)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	ItLogsAndDoesNotAckTheTask := func() {
		It("logs an error", func() {
			Eventually(logger).Should(gbytes.Say("disaster"))
		})

		It("does not acknowledge the task", func() {
			Consistently(task.AckCallCount).Should(BeZero())
		})
	}

	Context("when we can fetch work from the queue successfully", func() {
		BeforeEach(func() {
			queue.DequeueReturns(task, nil)
		})

		It("runs the job", func() {
			Eventually(job.RunCallCount).Should(BeNumerically(">=", 1))
		})

		It("acknowledges the task", func() {
			Eventually(task.AckCallCount).Should(BeNumerically(">=", 1))
		})

		It("measures the time taken to run the job", func() {
			Eventually(fakeTimer.TimeCallCount).Should(BeNumerically(">=", 1))
		})

		Context("when we can't build the job into something runnable", func() {
			BeforeEach(func() {
				foreman.BuildJobReturns(nil, errors.New("disaster"))
			})

			ItLogsAndDoesNotAckTheTask()

			It("emits a count metric for failed jobs", func() {
				Eventually(failedJobs.IncCallCount).Should(BeNumerically(">", 1))
			})
		})

		Context("when the job fails to run", func() {
			BeforeEach(func() {
				job.RunReturns(errors.New("disaster"))
			})

			ItLogsAndDoesNotAckTheTask()

			It("emits a count metric for failed jobs", func() {
				Eventually(failedJobs.IncCallCount).Should(BeNumerically(">", 1))
			})
		})

		Context("when acknowledging the job fails", func() {
			BeforeEach(func() {
				task.AckReturns(errors.New("disaster"))
			})

			It("logs an error", func() {
				Eventually(logger).Should(gbytes.Say("disaster"))
			})

			It("emits a count metric for failed acks", func() {
				Eventually(failedAcks.IncCallCount).Should(BeNumerically(">", 1))
			})
		})
	})

	Context("when retrieving a task from the queue returns an error", func() {
		BeforeEach(func() {
			queue.DequeueReturns(nil, errors.New("disaster"))
		})

		ItLogsAndDoesNotAckTheTask()
	})
})

package worker_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"cred-alert/worker"
)

var _ = Describe("Worker", func() {
	var (
		logger    *lagertest.TestLogger
		foreman   *queuefakes.FakeForeman
		fakeQueue *queuefakes.FakeQueue

		process ifrit.Process
		job     *queuefakes.FakeJob
		task    *queuefakes.FakeAckTask
		runner  ifrit.Runner
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("worker")
		foreman = &queuefakes.FakeForeman{}
		job = &queuefakes.FakeJob{}
		job.RunReturns(nil)
		task = &queuefakes.FakeAckTask{}
		foreman.BuildJobReturns(job, nil)
		fakeQueue = &queuefakes.FakeQueue{}
	})

	JustBeforeEach(func() {
		runner = worker.New(logger, foreman, fakeQueue)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Context("Everything's working fine", func() {
		BeforeEach(func() {
			fakeQueue.DequeueStub = func() (queue.AckTask, error) {
				// comparing with 1 becase call count was already incremented by this point
				for fakeQueue.DequeueCallCount() > 1 {
				}
				return task, nil
			}
		})

		It("Runs the job", func() {
			Eventually(job.RunCallCount).Should(Equal(1))
		})

		It("Acks the task", func() {
			Eventually(task.AckCallCount).Should(Equal(1))
		})
	})

	Context("Dequeue returns an error", func() {
		BeforeEach(func() {
			fakeQueue.DequeueStub = func() (queue.AckTask, error) {
				// comparing with 1 becase call count was already incremented by this point
				for fakeQueue.DequeueCallCount() > 1 {
				}
				return nil, errors.New("error dequeuing")
			}
		})

		It("logs an error", func() {
			Eventually(logger.LogMessages).Should(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("got-error"))
		})

	})
})

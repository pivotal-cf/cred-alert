package worker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"cred-alert/queue/queuefakes"
	"cred-alert/worker"
)

var _ = Describe("Worker", func() {
	var (
		logger  *lagertest.TestLogger
		foreman *queuefakes.FakeForeman
		queue   *queuefakes.FakeQueue

		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("worker")
		foreman = &queuefakes.FakeForeman{}
		queue = &queuefakes.FakeQueue{}

		job := &queuefakes.FakeJob{}
		job.RunReturns(nil)

		task := &queuefakes.FakeAckTask{}
		queue.DequeueReturns(task, nil)

		foreman.BuildJobReturns(job, nil)
	})

	JustBeforeEach(func() {
		runner := worker.New(logger, foreman, queue)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("works", func() {
		Expect(true).To(BeTrue())
	})
})

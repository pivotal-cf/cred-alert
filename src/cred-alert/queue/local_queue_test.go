package queue_test

import (
	"cred-alert/queue"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("CrappyQueue", func() {
	var (
		logger *lagertest.TestLogger
		localQ queue.Queue
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("local-queue")
	})

	JustBeforeEach(func() {
		localQ = queue.NewLocalQueue(logger)
	})

	It("queues and dequeues properly", func() {
		expectedTask := &dummyTask{}

		var err error
		err = localQ.Enqueue(expectedTask)
		Expect(err).ToNot(HaveOccurred())

		var task queue.Task
		task, err = localQ.Dequeue()
		Expect(err).ToNot(HaveOccurred())
		Expect(task).To(Equal(expectedTask))
	})

	It("returns error when trying to dequeue empty queue", func() {
		_, err := localQ.Dequeue()
		Expect(err).To(HaveOccurred())

		_, isEmpty := err.(queue.EmptyQueueError)
		Expect(isEmpty).To(BeTrue())
	})

})

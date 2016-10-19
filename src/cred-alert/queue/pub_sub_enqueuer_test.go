package queue_test

import (
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	"cloud.google.com/go/pubsub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PubSubEnqueuer", func() {
	var (
		logger   *lagertest.TestLogger
		topic    *queuefakes.FakeTopic
		task     *queuefakes.FakeTask
		enqueuer queue.Enqueuer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("pubsub-enqueuer")
		topic = &queuefakes.FakeTopic{}
		task = &queuefakes.FakeTask{}
		task.IDReturns("some-id")
		task.TypeReturns("some-type")
		task.PayloadReturns("some-payload")

		enqueuer = queue.NewPubSubEnqueuer(logger, topic)
	})

	It("tries to publish", func() {
		err := enqueuer.Enqueue(task)
		Expect(err).NotTo(HaveOccurred())

		Expect(topic.PublishCallCount()).To(Equal(1))
		_, message := topic.PublishArgsForCall(0)
		Expect(message).To(ConsistOf(&pubsub.Message{
			Attributes: map[string]string{
				"id":   "some-id",
				"type": "some-type",
			},
			Data: []byte("some-payload"),
		}))
	})

	Context("when publishing fails", func() {
		BeforeEach(func() {
			topic.PublishReturns([]string{}, errors.New("an-error"))
		})

		It("returns an error", func() {
			err := enqueuer.Enqueue(task)
			Expect(err).To(HaveOccurred())
		})
	})
})

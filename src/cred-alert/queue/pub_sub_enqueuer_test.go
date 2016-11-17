package queue_test

import (
	"cred-alert/crypto/cryptofakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"errors"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PubSubEnqueuer", func() {
	var (
		logger   *lagertest.TestLogger
		topic    *queuefakes.FakeTopic
		task     *queuefakes.FakeTask
		enqueuer queue.Enqueuer
		signer   *cryptofakes.FakeSigner
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("pubsub-enqueuer")
		topic = &queuefakes.FakeTopic{}
		task = &queuefakes.FakeTask{}
		task.IDReturns("some-id")
		task.TypeReturns("some-type")
		task.PayloadReturns("some-payload")
		signer = &cryptofakes.FakeSigner{}
		signer.SignReturns([]byte("some-signature"), nil)

		enqueuer = queue.NewPubSubEnqueuer(logger, topic, signer)
	})

	It("tries to publish", func() {
		err := enqueuer.Enqueue(task)
		Expect(err).NotTo(HaveOccurred())

		Expect(topic.PublishCallCount()).To(Equal(1))
		_, message := topic.PublishArgsForCall(0)
		Expect(message).To(ConsistOf(&pubsub.Message{
			Attributes: map[string]string{
				"id":        "some-id",
				"type":      "some-type",
				"signature": "c29tZS1zaWduYXR1cmU=",
			},
			Data: []byte("some-payload"),
		}))
	})

	It("signs the message", func() {
		enqueuer.Enqueue(task)

		Expect(signer.SignCallCount()).To(Equal(1))

		message := signer.SignArgsForCall(0)
		Expect(message).To(Equal([]byte("some-payload")))
	})

	Context("when signing fails", func() {
		var signingErr error

		BeforeEach(func() {
			signingErr = errors.New("My Special Error")
			signer.SignReturns([]byte{}, signingErr)
		})

		It("returns an error", func() {
			err := enqueuer.Enqueue(task)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signingErr))
		})
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

package queue_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"cred-alert/metrics/metricsfakes"
	"cred-alert/pubsubrunner"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = XDescribe("PubSubSubscriber", func() {
	var (
		logger        *lagertest.TestLogger
		firstMessage  *pubsub.Message
		secondMessage *pubsub.Message
		processor     *queuefakes.FakePubSubProcessor
		subscription  *pubsub.Subscription
		topic         *pubsub.Topic
		client        *pubsub.Client
		emitter       *metricsfakes.FakeEmitter

		psRunner *pubsubrunner.Runner
		runner   ifrit.Runner
		process  ifrit.Process
	)

	BeforeEach(func() {
		psRunner = &pubsubrunner.Runner{}
		psRunner.Setup()

		logger = lagertest.NewTestLogger("processor")

		firstMessage = &pubsub.Message{
			Attributes: map[string]string{
				"id": "some-id",
			},
		}

		secondMessage = &pubsub.Message{
			Attributes: map[string]string{
				"id": "some-other-id",
			},
		}

		var err error
		ctx := context.Background()
		client, err = pubsub.NewClient(ctx, "a-project-id")
		Expect(err).NotTo(HaveOccurred())

		topic, err = client.CreateTopic(ctx, "a-topic-id")
		Expect(err).NotTo(HaveOccurred())

		subscription, err = client.CreateSubscription(ctx, "a-subscription-id", pubsub.SubscriptionConfig{
			Topic: topic,
		})
		Expect(err).NotTo(HaveOccurred())

		res := topic.Publish(ctx, firstMessage)
		_, err = res.Get(ctx)
		Expect(err).NotTo(HaveOccurred())

		res = topic.Publish(ctx, secondMessage)
		_, err = res.Get(ctx)
		Expect(err).NotTo(HaveOccurred())

		processor = &queuefakes.FakePubSubProcessor{}

		emitter = &metricsfakes.FakeEmitter{}
		emitter.CounterReturns(&metricsfakes.FakeCounter{})

		fakeTimer := &metricsfakes.FakeTimer{}
		fakeTimer.TimeStub = func(logger lager.Logger, f func(), tags ...string) {
			f()
		}
		emitter.TimerReturns(fakeTimer)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		client.Close()
		psRunner.Teardown()
	})

	JustBeforeEach(func() {
		runner = queue.NewPubSubSubscriber(logger, subscription, processor, emitter)
		process = ginkgomon.Invoke(runner)
	})

	Context("when the runner is signaled", func() {
		It("exits gracefully", func() {
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive())
		})

		It("does not process any more messages", func() {
			Eventually(processor.ProcessCallCount).Should(Equal(2))
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive())

			res := topic.Publish(context.Background(), firstMessage)
			_, err := res.Get(context.Background())
			Expect(err).NotTo(HaveOccurred())

			Consistently(processor.ProcessCallCount).Should(Equal(2))
		})
	})

	It("tries to process the messages", func() {
		Eventually(processor.ProcessCallCount).Should(Equal(2))

		_, message1 := processor.ProcessArgsForCall(0)
		_, message2 := processor.ProcessArgsForCall(1)

		attributes := []map[string]string{
			message1.Attributes,
			message2.Attributes,
		}

		Expect(attributes).To(ConsistOf(firstMessage.Attributes, secondMessage.Attributes))
	})
})

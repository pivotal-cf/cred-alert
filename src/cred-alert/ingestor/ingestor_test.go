package ingestor_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/ingestor"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"

	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Ingestor", func() {
	var (
		in ingestor.Ingestor

		emitter   *metricsfakes.FakeEmitter
		taskQueue *queuefakes.FakeQueue
		generator *queuefakes.FakeUUIDGenerator

		logger *lagertest.TestLogger

		scan ingestor.PushScan

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter

		ingestErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("event-handler")
		emitter = &metricsfakes.FakeEmitter{}
		taskQueue = &queuefakes.FakeQueue{}
		generator = &queuefakes.FakeUUIDGenerator{}

		requestCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.ingestor_requests":
				return requestCounter
			case "cred_alert.ignored_events":
				return ignoredEventCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}

		scan = ingestor.PushScan{
			Owner:      "owner",
			Repository: "repo",
			To:         "to",
			From:       "from",
			Private:    true,
		}

		generator.GenerateReturns("id-1")
	})

	JustBeforeEach(func() {
		in = ingestor.NewIngestor(taskQueue, emitter, "cred_alert", generator)
		ingestErr = in.IngestPushScan(logger, scan, "github-id")
	})

	It("tries to enqueue a PushEventPlan", func() {
		Expect(ingestErr).NotTo(HaveOccurred())

		Expect(taskQueue.EnqueueCallCount()).To(Equal(1))

		expectedTask1 := queue.PushEventPlan{
			Owner:      "owner",
			Repository: "repo",
			To:         "to",
			From:       "from",
			Private:    true,
		}.Task("id-1")

		builtTask := taskQueue.EnqueueArgsForCall(0)
		Expect(builtTask).To(Equal(expectedTask1))
	})

	It("does not return an error", func() {
		Expect(ingestErr).NotTo(HaveOccurred())
	})

	It("increments cred_alert.ingestor_requests", func() {
		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	It("logs the correlation between our ID and a GitHub ID", func() {
		contents := logger.Buffer().Contents()

		Expect(contents).To(ContainSubstring("enqueuing-task"))
		Expect(contents).To(ContainSubstring("id-1"))
		Expect(contents).To(ContainSubstring("github-id"))
	})

	Context("when enqueuing a task fails", func() {
		BeforeEach(func() {
			taskQueue.EnqueueReturns(errors.New("disaster"))
		})

		It("returns an error", func() {
			Expect(ingestErr).To(HaveOccurred())
		})
	})

})

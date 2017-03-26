package ingestor_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/ingestor"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("Ingestor", func() {
	var (
		subject   ingestor.Ingestor
		logger    *lagertest.TestLogger
		fakeQueue *queuefakes.FakeEnqueuer
		pushScan  ingestor.PushScan
	)

	BeforeEach(func() {
		fakeQueue = &queuefakes.FakeEnqueuer{}

		uuidGenerator := &queuefakes.FakeUUIDGenerator{}
		uuidGenerator.GenerateReturns("my-special-uuid")

		emitter := &metricsfakes.FakeEmitter{}
		emitter.CounterReturns(&metricsfakes.FakeCounter{})

		logger = lagertest.NewTestLogger("ingestor")

		subject = ingestor.NewIngestor(
			fakeQueue,
			emitter,
			"metricPrefix",
			uuidGenerator,
		)

		t := time.Date(2017, 2, 27, 15, 20, 42, 0, time.UTC)
		pushScan = ingestor.PushScan{
			Owner:      "owner",
			Repository: "repo",
			PushTime:   t,
		}
	})

	It("queues up the message", func() {
		err := subject.IngestPushScan(logger, pushScan)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeQueue.EnqueueCallCount()).To(Equal(1))

		expectedJson := `{
			"owner":"owner",
			"repository":"repo",
			"pushTime":"2017-02-27T15:20:42Z"
		}`

		task := fakeQueue.EnqueueArgsForCall(0)
		Expect(task.ID()).To(Equal("my-special-uuid"))
		Expect(task.Type()).To(Equal("push-event"))
		Expect(task.Payload()).To(MatchJSON(expectedJson))
	})

	It("errors when queueing the message fails", func() {
		fakeQueue.EnqueueReturns(errors.New("Oh No!"))

		err := subject.IngestPushScan(logger, pushScan)
		Expect(err).To(HaveOccurred())
	})
})

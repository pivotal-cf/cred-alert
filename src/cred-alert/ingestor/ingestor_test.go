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

	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Ingestor", func() {
	var (
		in ingestor.Ingestor

		emitter   *metricsfakes.FakeEmitter
		taskQueue *queuefakes.FakeQueue
		whitelist *ingestor.Whitelist

		logger *lagertest.TestLogger

		scan ingestor.PushScan

		orgName   string
		repoName  string
		commitRef string

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter
	)

	BeforeEach(func() {
		orgName = "rad-co"
		repoName = "my-awesome-repo"
		commitRef = "refs/head/my-branch"

		logger = lagertest.NewTestLogger("event-handler")
		emitter = &metricsfakes.FakeEmitter{}
		taskQueue = &queuefakes.FakeQueue{}

		requestCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		whitelist = ingestor.BuildWhitelist()

		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.webhook_hits":
				return requestCounter
			case "cred_alert.ignored_events":
				return ignoredEventCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}

		scan = ingestor.PushScan{
			Owner:      orgName,
			Repository: repoName,
			Ref:        commitRef,

			Diffs: []ingestor.PushScanDiff{
				{From: "commit-1", To: "commit-2"},
				{From: "commit-2", To: "commit-3"},
				{From: "commit-3", To: "commit-4"},
			},
		}
	})

	JustBeforeEach(func() {
		in = ingestor.NewIngestor(taskQueue, emitter, whitelist)
	})

	Describe("enqueuing tasks in the queue", func() {
		It("enqueues tasks in the queue", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskQueue.EnqueueCallCount()).To(Equal(3))

			expectedTask1 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Ref:        commitRef,
				From:       "commit-1",
				To:         "commit-2",
			}.Task()

			builtTask := taskQueue.EnqueueArgsForCall(0)
			Expect(builtTask).To(Equal(expectedTask1))

			expectedTask2 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Ref:        commitRef,
				From:       "commit-2",
				To:         "commit-3",
			}.Task()

			builtTask = taskQueue.EnqueueArgsForCall(1)
			Expect(builtTask).To(Equal(expectedTask2))

			expectedTask3 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Ref:        commitRef,
				From:       "commit-3",
				To:         "commit-4",
			}.Task()

			builtTask = taskQueue.EnqueueArgsForCall(2)
			Expect(builtTask).To(Equal(expectedTask3))
		})

		Context("when enqueuing a task fails", func() {
			BeforeEach(func() {
				taskQueue.EnqueueReturns(errors.New("disaster"))
			})

			It("returns an error", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	It("emits count when it is invoked", func() {
		err := in.IngestPushScan(logger, scan)
		Expect(err).NotTo(HaveOccurred())

		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	Context("when it has a whitelist of ignored repos", func() {
		BeforeEach(func() {
			whitelist = ingestor.BuildWhitelist(repoName)
		})

		It("ignores patterns in whitelist", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskQueue.EnqueueCallCount()).To(BeZero())

			Expect(logger.LogMessages()).To(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("ignored-repo"))
			Expect(logger.Logs()[0].Data["repo"]).To(Equal(repoName))
		})

		It("emits a count of ignored push events", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).NotTo(HaveOccurred())

			Expect(ignoredEventCounter.IncCallCount()).To(Equal(1))
		})
	})
})

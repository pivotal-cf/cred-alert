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

var _ = Describe("EventHandler", func() {
	var (
		in ingestor.Ingestor

		foreman   *queuefakes.FakeForeman
		emitter   *metricsfakes.FakeEmitter
		taskQueue *queuefakes.FakeQueue
		whitelist *ingestor.Whitelist

		logger *lagertest.TestLogger

		scan ingestor.PushScan
		job  *queuefakes.FakeJob

		orgName  string
		repoName string

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter
	)

	BeforeEach(func() {
		orgName = "rad-co"
		repoName = "my-awesome-repo"

		logger = lagertest.NewTestLogger("event-handler")
		emitter = &metricsfakes.FakeEmitter{}
		taskQueue = &queuefakes.FakeQueue{}

		requestCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		whitelist = ingestor.BuildWhitelist()

		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.webhook_requests":
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

			Diffs: []ingestor.PushScanDiff{
				{Start: "commit-1", End: "commit-2"},
				{Start: "commit-2", End: "commit-3"},
				{Start: "commit-3", End: "commit-4"},
			},
		}

		job = &queuefakes.FakeJob{}

		foreman = &queuefakes.FakeForeman{}
		foreman.BuildJobReturns(job, nil)
	})

	JustBeforeEach(func() {
		in = ingestor.NewIngestor(foreman, taskQueue, emitter, whitelist)
	})

	Describe("enqueuing tasks in the queue", func() {
		It("enqueues tasks in the queue", func() {
			in.IngestPushScan(logger, scan)

			Expect(taskQueue.EnqueueCallCount()).To(Equal(3))

			expectedTask1 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-1",
				End:        "commit-2",
			}.Task()

			builtTask := taskQueue.EnqueueArgsForCall(0)
			Expect(builtTask).To(Equal(expectedTask1))

			expectedTask2 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-2",
				End:        "commit-3",
			}.Task()

			builtTask = taskQueue.EnqueueArgsForCall(1)
			Expect(builtTask).To(Equal(expectedTask2))

			expectedTask3 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-3",
				End:        "commit-4",
			}.Task()

			builtTask = taskQueue.EnqueueArgsForCall(2)
			Expect(builtTask).To(Equal(expectedTask3))
		})
	})

	Describe("running the jobs directly", func() {
		It("enqueues tasks in the queue", func() {
			in.IngestPushScan(logger, scan)

			Expect(foreman.BuildJobCallCount()).Should(Equal(3))
			Expect(job.RunCallCount()).Should(Equal(3))

			expectedTask1 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-1",
				End:        "commit-2",
			}.Task()

			builtTask := foreman.BuildJobArgsForCall(0)
			Expect(builtTask).To(Equal(expectedTask1))

			expectedTask2 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-2",
				End:        "commit-3",
			}.Task()

			builtTask = foreman.BuildJobArgsForCall(1)
			Expect(builtTask).To(Equal(expectedTask2))

			expectedTask3 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-3",
				End:        "commit-4",
			}.Task()

			builtTask = foreman.BuildJobArgsForCall(2)
			Expect(builtTask).To(Equal(expectedTask3))
		})

		Context("when the queue fails to queue something", func() {
			BeforeEach(func() {
				taskQueue.EnqueueReturns(errors.New("disaster"))
			})

			It("still tries to run them directly because queueing isn't prime time just yet", func() {
				in.IngestPushScan(logger, scan)

				Expect(foreman.BuildJobCallCount()).Should(Equal(3))
				Expect(job.RunCallCount()).Should(Equal(3))
			})
		})
	})

	It("emits count when it is invoked", func() {
		in.IngestPushScan(logger, scan)

		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	Context("when it has a whitelist of ignored repos", func() {
		BeforeEach(func() {
			whitelist = ingestor.BuildWhitelist(repoName)
		})

		It("ignores patterns in whitelist", func() {
			in.IngestPushScan(logger, scan)

			Expect(taskQueue.EnqueueCallCount()).To(BeZero())

			Expect(logger.LogMessages()).To(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("ignored-repo"))
			Expect(logger.Logs()[0].Data["repo"]).To(Equal(repoName))
		})

		It("emits a count of ignored push events", func() {
			in.IngestPushScan(logger, scan)
			Expect(ignoredEventCounter.IncCallCount()).To(Equal(1))
		})
	})
})

package webhook_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"cred-alert/webhook"

	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("EventHandler", func() {
	var (
		eventHandler webhook.EventHandler
		logger       *lagertest.TestLogger
		emitter      *metricsfakes.FakeEmitter

		foreman *queuefakes.FakeForeman
		job     *queuefakes.FakeJob

		orgName      string
		repoName     string
		repoFullName string

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter

		whitelist *webhook.Whitelist
		scan      webhook.PushScan
	)

	BeforeEach(func() {
		orgName = "rad-co"
		repoName = "my-awesome-repo"
		repoFullName = fmt.Sprintf("%s/%s", orgName, repoName)

		emitter = &metricsfakes.FakeEmitter{}
		requestCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		whitelist = webhook.BuildWhitelist()

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

		logger = lagertest.NewTestLogger("event-handler")

		scan = webhook.PushScan{
			Owner:      orgName,
			Repository: repoName,

			Diffs: []webhook.PushScanDiff{
				{Start: "commit-1", End: "commit-2"},
			},
		}

		job = &queuefakes.FakeJob{}

		foreman = &queuefakes.FakeForeman{}
		foreman.BuildJobReturns(job, nil)
	})

	JustBeforeEach(func() {
		eventHandler = webhook.NewEventHandler(foreman, emitter, whitelist)
	})

	Context("when there are multiple commits in a single event", func() {
		BeforeEach(func() {
			diffs := []webhook.PushScanDiff{
				{Start: "commit-1", End: "commit-2"},
				{Start: "commit-2", End: "commit-3"},
				{Start: "commit-3", End: "commit-4"},
			}

			scan.Diffs = diffs
		})

		It("compares each commit individually", func() {
			eventHandler.HandleEvent(logger, scan)

			Expect(foreman.BuildJobCallCount()).To(Equal(3))
			Expect(job.RunCallCount()).To(Equal(3))

			expectedPlan1 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-1",
				End:        "commit-2",
			}

			builtTask := foreman.BuildJobArgsForCall(0)
			Expect(builtTask).To(Equal(expectedPlan1.Task()))

			expectedPlan2 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-2",
				End:        "commit-3",
			}

			builtTask = foreman.BuildJobArgsForCall(1)
			Expect(builtTask).To(Equal(expectedPlan2.Task()))

			expectedPlan3 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				Start:      "commit-3",
				End:        "commit-4",
			}

			builtTask = foreman.BuildJobArgsForCall(2)
			Expect(builtTask).To(Equal(expectedPlan3.Task()))
		})
	})

	It("emits count when it is invoked", func() {
		eventHandler.HandleEvent(logger, scan)

		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	Context("when it has a whitelist of ignored repos", func() {
		BeforeEach(func() {
			whitelist = webhook.BuildWhitelist(repoName)
		})

		It("ignores patterns in whitelist", func() {
			eventHandler.HandleEvent(logger, scan)

			Expect(foreman.BuildJobCallCount()).To(BeZero())

			Expect(logger.LogMessages()).To(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("ignored-repo"))
			Expect(logger.Logs()[0].Data["repo"]).To(Equal(repoName))
		})

		It("emits a count of ignored push events", func() {
			eventHandler.HandleEvent(logger, scan)
			Expect(ignoredEventCounter.IncCallCount()).To(Equal(1))
		})
	})
})

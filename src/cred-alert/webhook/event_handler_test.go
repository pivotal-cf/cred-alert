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

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Extract", func() {
	var (
		event github.PushEvent
	)

	BeforeEach(func() {
		event = github.PushEvent{
			Before: github.String("commit-sha-0"),
			Repo: &github.PushEventRepository{
				Name: github.String("repository-name"),
				Owner: &github.PushEventRepoOwner{
					Name: github.String("repository-owner"),
				},
			},
			Commits: []github.PushEventCommit{
				{ID: github.String("commit-sha-1")},
				{ID: github.String("commit-sha-2")},
				{ID: github.String("commit-sha-3")},
				{ID: github.String("commit-sha-4")},
				{ID: github.String("commit-sha-5")},
			},
		}
	})

	It("can give us the first and last commit of the push", func() {
		scan, valid := webhook.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.FirstCommit()).To(Equal("commit-sha-0"))
		Expect(scan.LastCommit()).To(Equal("commit-sha-5"))
	})

	It("can extract a value object from a github push event", func() {
		scan, valid := webhook.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))
		Expect(scan.Diffs).To(Equal([]webhook.PushScanDiff{
			{Start: "commit-sha-0", End: "commit-sha-1"},
			{Start: "commit-sha-1", End: "commit-sha-2"},
			{Start: "commit-sha-2", End: "commit-sha-3"},
			{Start: "commit-sha-3", End: "commit-sha-4"},
			{Start: "commit-sha-4", End: "commit-sha-5"},
		}))
	})

	It("can have a full repository name", func() {
		scan, valid := webhook.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))

		Expect(scan.FullRepoName()).To(Equal("repository-owner/repository-name"))
	})

	It("can handle if there are no commits in a push (may not even be possible)", func() {
		event.Commits = []github.PushEventCommit{}

		_, valid := webhook.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if there is no before specified", func() {
		event.Before = nil

		_, valid := webhook.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if this is the initial push to the repository because the GitHub API doesn't allow this comparison", func() {
		event.Before = github.String("0000000000000000000000000000000000000000")

		_, valid := webhook.Extract(event)
		Expect(valid).To(BeFalse())
	})
})

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

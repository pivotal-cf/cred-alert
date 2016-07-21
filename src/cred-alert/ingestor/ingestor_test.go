package ingestor_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"cred-alert/ingestor"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/models/modelsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"

	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Ingestor", func() {
	var (
		in ingestor.Ingestor

		emitter          *metricsfakes.FakeEmitter
		taskQueue        *queuefakes.FakeQueue
		whitelist        *ingestor.Whitelist
		generator        *queuefakes.FakeUUIDGenerator
		commitRepository *modelsfakes.FakeCommitRepository

		logger *lagertest.TestLogger

		scan ingestor.PushScan

		orgName             string
		repoName            string
		commitRef           string
		time2, time3, time4 time.Time

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter
	)

	BeforeEach(func() {
		orgName = "rad-co"
		repoName = "my-awesome-repo"
		commitRef = "refs/heads/my-branch"
		time2 = time.Now()
		time3 = time2.Add(time.Minute)
		time4 = time3.Add(time.Minute)

		logger = lagertest.NewTestLogger("event-handler")
		emitter = &metricsfakes.FakeEmitter{}
		taskQueue = &queuefakes.FakeQueue{}
		generator = &queuefakes.FakeUUIDGenerator{}
		commitRepository = &modelsfakes.FakeCommitRepository{}
		commitRepository.IsRepoRegisteredReturns(true, nil)

		requestCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		whitelist = ingestor.BuildWhitelist()

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
			Owner:      orgName,
			Repository: repoName,
			Ref:        commitRef,

			Diffs: []ingestor.PushScanDiff{
				{From: "commit-1", To: "commit-2", ToTimestamp: time2},
				{From: "commit-2", To: "commit-3", ToTimestamp: time3},
				{From: "commit-3", To: "commit-4", ToTimestamp: time4},
			},
		}

		callCount := 0
		generator.GenerateStub = func() string {
			callCount++
			return fmt.Sprintf("id-%d", callCount)
		}
	})

	JustBeforeEach(func() {
		in = ingestor.NewIngestor(taskQueue, emitter, whitelist, generator, commitRepository)
	})

	Describe("enqueuing tasks in the queue", func() {
		It("enqueues tasks in the queue", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskQueue.EnqueueCallCount()).To(Equal(3))

			expectedTask1 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				From:       "commit-1",
				To:         "commit-2",
			}.Task("id-1")

			builtTask := taskQueue.EnqueueArgsForCall(0)
			Expect(builtTask).To(Equal(expectedTask1))

			expectedTask2 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				From:       "commit-2",
				To:         "commit-3",
			}.Task("id-2")

			builtTask = taskQueue.EnqueueArgsForCall(1)
			Expect(builtTask).To(Equal(expectedTask2))

			expectedTask3 := queue.DiffScanPlan{
				Owner:      orgName,
				Repository: repoName,
				From:       "commit-3",
				To:         "commit-4",
			}.Task("id-3")

			builtTask = taskQueue.EnqueueArgsForCall(2)
			Expect(builtTask).To(Equal(expectedTask3))
		})

		It("saves queued commits to the database", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).ToNot(HaveOccurred())
			Expect(commitRepository.RegisterCommitCallCount()).To(Equal(3))

			_, commit1 := commitRepository.RegisterCommitArgsForCall(0)
			Expect(commit1.SHA).To(Equal("commit-2"))
			Expect(commit1.Timestamp).To(Equal(time2))
			Expect(commit1.Repository).To(Equal(repoName))
			Expect(commit1.Owner).To(Equal(orgName))

			_, commit2 := commitRepository.RegisterCommitArgsForCall(1)
			Expect(commit2.SHA).To(Equal("commit-3"))
			Expect(commit2.Timestamp).To(Equal(time3))
			Expect(commit2.Repository).To(Equal(repoName))
			Expect(commit2.Owner).To(Equal(orgName))

			_, commit3 := commitRepository.RegisterCommitArgsForCall(2)
			Expect(commit3.SHA).To(Equal("commit-4"))
			Expect(commit3.Timestamp).To(Equal(time4))
			Expect(commit3.Repository).To(Equal(repoName))
			Expect(commit3.Owner).To(Equal(orgName))
		})

		Context("when the from commit is the initial nil commit", func() {
			BeforeEach(func() {
				initialCommitParentHash := "0000000000000000000000000000000000000000"
				scan.Diffs[0].From = initialCommitParentHash
			})

			It("queues a ref-scan", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).NotTo(HaveOccurred())

				Expect(taskQueue.EnqueueCallCount()).To(Equal(3))

				expectedTask := queue.RefScanPlan{
					Owner:      orgName,
					Repository: repoName,
					Ref:        "commit-2",
				}.Task("id-1")

				builtTask := taskQueue.EnqueueArgsForCall(0)
				Expect(builtTask).To(Equal(expectedTask))
			})
		})

		Context("when enqueuing a task fails", func() {
			BeforeEach(func() {
				taskQueue.EnqueueReturns(errors.New("disaster"))
			})

			It("returns an error", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).To(HaveOccurred())
			})

			It("should not save commits to db", func() {
				in.IngestPushScan(logger, scan)
				Expect(commitRepository.RegisterCommitCallCount()).To(Equal(0))
			})
		})

		Context("when the repo hasn't been registered", func() {
			BeforeEach(func() {
				commitRepository.IsRepoRegisteredReturns(false, nil)
			})

			It("queues a ref scan", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).NotTo(HaveOccurred())

				Expect(taskQueue.EnqueueCallCount()).To(Equal(4))

				expectedTask1 := queue.RefScanPlan{
					Owner:      orgName,
					Repository: repoName,
					Ref:        "commit-1",
				}.Task("id-1")

				builtTask := taskQueue.EnqueueArgsForCall(0)
				Expect(builtTask).To(Equal(expectedTask1))
			})

			It("log when queuing ref scan happens", func() {
				in.IngestPushScan(logger, scan)
				Expect(logger).To(gbytes.Say("enqueue-ref-scan-succeeded"))
			})

			Context("When queuing a ref-scan fails", func() {
				var expectedError error

				BeforeEach(func() {
					expectedError = errors.New("some-error")
					taskQueue.EnqueueReturns(expectedError)
				})

				It("logs on error", func() {
					err := in.IngestPushScan(logger, scan)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(expectedError))
					Expect(logger).To(gbytes.Say("enqueue-ref-scan-failed"))
				})
			})
		})

		Context("when checking for repo existence fails", func() {
			var findError error

			BeforeEach(func() {
				findError = errors.New("some-error")
				commitRepository.IsRepoRegisteredReturns(true, findError)
			})

			It("logs an error", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("Error checking for repo"))
			})

			It("queues a ref scan", func() {
				err := in.IngestPushScan(logger, scan)
				Expect(err).NotTo(HaveOccurred())

				Expect(taskQueue.EnqueueCallCount()).To(Equal(4))

				expectedTask1 := queue.RefScanPlan{
					Owner:      orgName,
					Repository: repoName,
					Ref:        "commit-1",
				}.Task("id-1")

				builtTask := taskQueue.EnqueueArgsForCall(0)
				Expect(builtTask).To(Equal(expectedTask1))
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

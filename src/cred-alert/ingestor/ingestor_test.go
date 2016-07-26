package ingestor_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"cred-alert/db/dbfakes"
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

		emitter          *metricsfakes.FakeEmitter
		taskQueue        *queuefakes.FakeQueue
		whitelist        *ingestor.Whitelist
		generator        *queuefakes.FakeUUIDGenerator
		commitRepository *dbfakes.FakeCommitRepository

		logger *lagertest.TestLogger

		scan ingestor.PushScan

		ownerName           string
		repoName            string
		commitRef           string
		time2, time3, time4 time.Time

		requestCounter      *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter
	)

	BeforeEach(func() {
		ownerName = "rad-co"
		repoName = "my-awesome-repo"
		commitRef = "refs/heads/my-branch"
		time2 = time.Now()
		time3 = time2.Add(time.Minute)
		time4 = time3.Add(time.Minute)

		logger = lagertest.NewTestLogger("event-handler")
		emitter = &metricsfakes.FakeEmitter{}
		taskQueue = &queuefakes.FakeQueue{}
		generator = &queuefakes.FakeUUIDGenerator{}
		commitRepository = &dbfakes.FakeCommitRepository{}
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
			Owner:      ownerName,
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
		It("enqueues an ancestry scan at the end commit", func() {
			err := in.IngestPushScan(logger, scan)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskQueue.EnqueueCallCount()).To(Equal(1))

			expectedTask1 := queue.AncestryScanPlan{
				Owner:      ownerName,
				Repository: repoName,
				SHA:        "commit-4",
				Depth:      queue.DefaultScanDepth,
			}.Task("id-1")

			builtTask := taskQueue.EnqueueArgsForCall(0)
			Expect(builtTask).To(Equal(expectedTask1))
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

				Expect(taskQueue.EnqueueCallCount()).To(Equal(2))

				expectedTask1 := queue.RefScanPlan{
					Owner:      ownerName,
					Repository: repoName,
					Ref:        "commit-1",
				}.Task("id-1")

				builtTask := taskQueue.EnqueueArgsForCall(0)
				Expect(builtTask).To(Equal(expectedTask1))
			})

			It("Saves the commit to the db", func() {
				in.IngestPushScan(logger, scan)

				Expect(commitRepository.RegisterCommitCallCount()).To(Equal(1))
				_, commit := commitRepository.RegisterCommitArgsForCall(0)
				Expect(commit.SHA).To(Equal("commit-1"))
				Expect(commit.Repository).To(Equal(repoName))
				Expect(commit.Owner).To(Equal(ownerName))
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

				It("Does not save the commit to the db", func() {
					in.IngestPushScan(logger, scan)
					Expect(commitRepository.RegisterCommitCallCount()).To(Equal(0))
				})
			})

			Context("when registering the commit fails", func() {
				BeforeEach(func() {
					commitRepository.RegisterCommitReturns(errors.New("an-error"))
				})

				It("returns an error", func() {
					err := in.IngestPushScan(logger, scan)
					Expect(err).To(HaveOccurred())
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

				Expect(taskQueue.EnqueueCallCount()).To(Equal(2))

				expectedTask1 := queue.RefScanPlan{
					Owner:      ownerName,
					Repository: repoName,
					Ref:        "commit-1",
				}.Task("id-1")

				builtTask := taskQueue.EnqueueArgsForCall(0)
				Expect(builtTask).To(Equal(expectedTask1))
			})

			It("Saves the commit to the db", func() {
				in.IngestPushScan(logger, scan)

				Expect(commitRepository.RegisterCommitCallCount()).To(Equal(1))
				_, commit := commitRepository.RegisterCommitArgsForCall(0)
				Expect(commit.SHA).To(Equal("commit-1"))
				Expect(commit.Repository).To(Equal(repoName))
				Expect(commit.Owner).To(Equal(ownerName))
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

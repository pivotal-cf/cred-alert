package queue_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/github/githubfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("Ancestry Scan Job", func() {
	var (
		logger *lagertest.TestLogger

		taskQueue            *queuefakes.FakeQueue
		client               *githubfakes.FakeClient
		commitRepository     *dbfakes.FakeCommitRepository
		maxDepthCounter      *metricsfakes.FakeCounter
		initialCommitCounter *metricsfakes.FakeCounter
		emitter              *metricsfakes.FakeEmitter
		generator            *queuefakes.FakeUUIDGenerator

		plan queue.AncestryScanPlan
		job  *queue.AncestryScanJob
	)

	BeforeEach(func() {
		plan = queue.AncestryScanPlan{
			Owner:      "owner",
			Repository: "repo",
			SHA:        "sha",
		}

		taskQueue = &queuefakes.FakeQueue{}
		client = &githubfakes.FakeClient{}
		commitRepository = &dbfakes.FakeCommitRepository{}
		emitter = &metricsfakes.FakeEmitter{}
		maxDepthCounter = &metricsfakes.FakeCounter{}
		initialCommitCounter = &metricsfakes.FakeCounter{}
		logger = lagertest.NewTestLogger("ancestry-scan")
		generator = &queuefakes.FakeUUIDGenerator{}
		emitter.CounterReturns(maxDepthCounter)

		emitter.CounterStub = func(name string) metrics.Counter {
			if name == "cred_alert.max-depth-reached" {
				return maxDepthCounter
			} else {
				return initialCommitCounter
			}
		}

		callCount := -1
		generator.GenerateStub = func() string {
			callCount++
			return fmt.Sprintf("id-%d", callCount)
		}
	})

	JustBeforeEach(func() {
		job = queue.NewAncestryScanJob(plan, commitRepository, client, emitter, taskQueue, generator)
	})

	var ItMarksTheCommitAsSeen = func() {
		It("marks the commit as seen", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(commitRepository.RegisterCommitCallCount()).To(Equal(1))
			_, registeredCommit := commitRepository.RegisterCommitArgsForCall(0)
			Expect(registeredCommit).To(Equal(&db.Commit{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				// TODO: timestamp
			}))
		})
	}

	var ItStopsAndDoesNotEnqueueAnyMoreWork = func() {
		It("stops and does not enqueue any more work", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskQueue.EnqueueCallCount()).To(BeZero())
			Expect(commitRepository.RegisterCommitCallCount()).To(BeZero())
		})
	}

	var ItReturnsAndLogsAnError = func(expectedError error) {
		It("returns and logs an error", func() {
			err := job.Run(logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedError))
			Expect(logger).To(gbytes.Say("scanning-ancestry.failed"))
		})
	}

	var ItDoesNotRegisterCommit = func() {
		It("returns and logs an error", func() {
			job.Run(logger)
			Expect(commitRepository.RegisterCommitCallCount()).To(BeZero())
		})
	}

	Describe("running the job", func() {
		Context("when the commit repository has an error finding a commit", func() {
			expectedError := errors.New("client repository error")

			BeforeEach(func() {
				commitRepository.IsCommitRegisteredReturns(false, expectedError)
			})

			ItReturnsAndLogsAnError(expectedError)
			ItDoesNotRegisterCommit()
		})

		Context("when we have not previously scanned the commit", func() {
			BeforeEach(func() {
				commitRepository.IsCommitRegisteredReturns(false, nil)
			})

			Context("when we have not reached the maximum scan depth", func() {
				BeforeEach(func() {
					plan.Depth = 5
				})

				Context("when the github client returns an error", func() {
					expectedError := errors.New("client error")

					BeforeEach(func() {
						client.ParentsReturns(nil, expectedError)
					})

					ItReturnsAndLogsAnError(expectedError)
					ItDoesNotRegisterCommit()
				})

				Context("when the commit has parents", func() {
					expectedParents := []string{
						"abcdef",
						"123456",
						"789aee",
					}

					BeforeEach(func() {
						client.ParentsReturns(expectedParents, nil)
					})

					Context("when the task queue returns an error enqueueing diffs", func() {
						expectedError := errors.New("queue error")

						BeforeEach(func() {
							taskQueue.EnqueueStub = func(task queue.Task) error {
								if task.Type() == queue.TaskTypeDiffScan {
									return expectedError
								}
								return nil
							}
						})

						ItReturnsAndLogsAnError(expectedError)
						ItDoesNotRegisterCommit()
					})

					It("scans the diffs between the current commit and its parents", func() {
						err := job.Run(logger)
						Expect(err).NotTo(HaveOccurred())

						Expect(taskQueue.EnqueueCallCount()).To(Equal(6))

						for i, parent := range expectedParents {
							expectedTask := queue.DiffScanPlan{
								Owner:      plan.Owner,
								Repository: plan.Repository,
								From:       parent,
								To:         plan.SHA,
							}.Task(fmt.Sprintf("id-%d", 2*i))
							task := taskQueue.EnqueueArgsForCall(2 * i)
							Expect(task).To(Equal(expectedTask))
						}
					})

					Context("when the task queue returns an error enqueueing ancestry scans", func() {
						expectedError := errors.New("disaster")
						BeforeEach(func() {
							taskQueue.EnqueueStub = func(task queue.Task) error {
								if task.Type() == queue.TaskTypeAncestryScan {
									return expectedError
								}
								return nil
							}
						})

						ItReturnsAndLogsAnError(expectedError)
						ItDoesNotRegisterCommit()
					})

					Context("when the commit repository returns an error registering the commit", func() {
						expectedError := errors.New("disaster")
						BeforeEach(func() {
							commitRepository.RegisterCommitReturns(expectedError)
						})

						ItReturnsAndLogsAnError(expectedError)
					})

					It("queues an ancestry-scan for each parent commit with one less depth", func() {
						err := job.Run(logger)
						Expect(err).NotTo(HaveOccurred())

						Expect(taskQueue.EnqueueCallCount()).To(Equal(6))

						for i, parent := range expectedParents {
							expectedTask := queue.AncestryScanPlan{
								Owner:           plan.Owner,
								Repository:      plan.Repository,
								CommitTimestamp: plan.CommitTimestamp,
								SHA:             parent,
								Depth:           plan.Depth - 1,
							}.Task(fmt.Sprintf("id-%d", 2*i+1))
							task := taskQueue.EnqueueArgsForCall(2*i + 1)
							Expect(task).To(Equal(expectedTask))
						}
					})

					ItMarksTheCommitAsSeen()
				})

				Context("when the current commit is the initial commit", func() {
					BeforeEach(func() {
						client.ParentsReturns([]string{}, nil)
					})

					It("Enqueues a ref scan", func() {
						err := job.Run(logger)
						Expect(err).ToNot(HaveOccurred())
						Expect(taskQueue.EnqueueCallCount()).To(Equal(1))

						task := taskQueue.EnqueueArgsForCall(0)
						Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
						Expect(task.Payload()).To(MatchJSON(`
							{
								"owner": "owner",
								"repository": "repo",
								"ref": "sha"
							}
						`))
					})

					It("increments the initial commit counter", func() {
						job.Run(logger)
						Expect(initialCommitCounter.IncCallCount()).To(Equal(1))
					})

					Context("when there's an error enqueuing the ref scan", func() {
						expectedError := errors.New("enqueue error")

						BeforeEach(func() {
							taskQueue.EnqueueReturns(expectedError)
						})

						ItReturnsAndLogsAnError(expectedError)
						ItDoesNotRegisterCommit()
					})
				})
			})

			Describe("reaching the maximum scan depth", func() {
				var ItHandlesHittingTheMaximumScanDepth = func() {
					It("enqueues a ref scan of the commit", func() {
						err := job.Run(logger)
						Expect(err).NotTo(HaveOccurred())

						Expect(taskQueue.EnqueueCallCount()).To(Equal(1))

						task := taskQueue.EnqueueArgsForCall(0)
						Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
						Expect(task.Payload()).To(MatchJSON(`
							{
								"owner": "owner",
								"repository": "repo",
								"ref": "sha"
							}
						`))
					})

					ItMarksTheCommitAsSeen()

					It("emits a counter saying that it ran out of depth", func() {
						job.Run(logger)
						Expect(maxDepthCounter.IncCallCount()).To(Equal(1))
					})

					It("logs that max depth was reached", func() {
						job.Run(logger)
						Expect(logger).To(gbytes.Say(`scanning-ancestry.max-depth-reached`))
					})

					It("does not look for any more parents", func() {
						Expect(client.ParentsCallCount()).To(Equal(0))
					})

					Context("When there is an error registering a commit", func() {
						expectedError := errors.New("disaster")
						BeforeEach(func() {
							commitRepository.RegisterCommitReturns(expectedError)
						})

						ItReturnsAndLogsAnError(expectedError)
					})

					Context("when there is an error enqueuing a ref scan", func() {
						expectedError := errors.New("disaster")

						BeforeEach(func() {
							taskQueue.EnqueueStub = func(task queue.Task) error {
								Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
								return expectedError
							}
						})

						ItReturnsAndLogsAnError(expectedError)
						ItDoesNotRegisterCommit()
					})
				}

				Context("when we have reached the maximum scan depth", func() {
					BeforeEach(func() {
						plan.Depth = 0
						// Fail if it tries to enqueue more tasks
						taskQueue.EnqueueStub = func(task queue.Task) error {
							Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
							return nil
						}
					})

					ItHandlesHittingTheMaximumScanDepth()
				})

				Context("when we have somehow(?!) gone beyond the maximum scan depth", func() {
					BeforeEach(func() {
						plan.Depth = -5
						// Fail if it tries to enqueue more tasks
						taskQueue.EnqueueStub = func(task queue.Task) error {
							Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
							return nil
						}
					})

					ItHandlesHittingTheMaximumScanDepth()
				})
			})
		})

		Context("when we have previously scanned the commit", func() {
			BeforeEach(func() {
				commitRepository.IsCommitRegisteredReturns(true, nil)
			})

			ItStopsAndDoesNotEnqueueAnyMoreWork()
		})

		Context("when there is an error checking if we have scanned the commit", func() {
			BeforeEach(func() {
				commitRepository.IsCommitRegisteredReturns(false, errors.New("disaster"))
			})

			It("stops and does not enqueue any more work", func() {
				err := job.Run(logger)
				Expect(err).To(MatchError("disaster"))

				Expect(taskQueue.EnqueueCallCount()).To(BeZero())
				Expect(commitRepository.RegisterCommitCallCount()).To(BeZero())
			})
		})
	})
})

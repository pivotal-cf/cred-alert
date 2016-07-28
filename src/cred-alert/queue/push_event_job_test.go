package queue_test

import (
	"cred-alert/db/dbfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("PushEventJob", func() {
	var (
		commitRepository *dbfakes.FakeCommitRepository
		taskQueue        *queuefakes.FakeQueue

		logger *lagertest.TestLogger
		job    *queue.PushEventJob
	)

	BeforeEach(func() {
		commitRepository = &dbfakes.FakeCommitRepository{}
		taskQueue = &queuefakes.FakeQueue{}

		plan := queue.PushEventPlan{
			Owner:      "owner",
			Repository: "repo",
			From:       "from",
			To:         "to",
		}
		id := "id"
		job = queue.NewPushEventJob(plan, id, taskQueue, commitRepository)
		logger = lagertest.NewTestLogger("push-event-job")
	})

	Describe("Run", func() {
		var runErr error
		JustBeforeEach(func() {
			runErr = job.Run(logger)
		})

		It("tries to determine if the repo is registered", func() {
			Expect(commitRepository.IsRepoRegisteredCallCount()).To(Equal(1))
		})

		Context("when the repo is not registered", func() {
			BeforeEach(func() {
				commitRepository.IsRepoRegisteredReturns(false, nil)
			})

			It("tries to enqueue a RefScanPlan", func() {
				Expect(taskQueue.EnqueueCallCount()).To(BeNumerically(">", 0))
				expectedTask := queue.RefScanPlan{
					Owner:      "owner",
					Repository: "repo",
					Ref:        "from",
				}.Task("id")
				actualTask := taskQueue.EnqueueArgsForCall(0)
				Expect(actualTask).To(Equal(expectedTask))
			})

			It("does not return an error", func() {
				Expect(runErr).NotTo(HaveOccurred())
			})

			Context("when enqueuing fails", func() {
				BeforeEach(func() {
					taskQueue.EnqueueReturns(errors.New("an-error"))
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
				})
			})
		})

		Context("when the repo is registered", func() {
			BeforeEach(func() {
				commitRepository.IsRepoRegisteredReturns(true, nil)
			})

			It("does not try to enqueue a RefScanPlan", func() {
				Expect(taskQueue.EnqueueCallCount()).To(Equal(1)) // still enqueues AncestryScanPlan
			})
		})

		Context("when trying to determine registeredness of the repo fails", func() {
			BeforeEach(func() {
				commitRepository.IsRepoRegisteredReturns(false, errors.New("an-error"))
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
			})
		})

		It("tries to enqueue an AncestryScanPlan", func() {
			Expect(taskQueue.EnqueueCallCount()).To(BeNumerically(">", 0))
			expectedTask := queue.AncestryScanPlan{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "to",
				Depth:      queue.DefaultScanDepth,
			}.Task("id")
			actualTask := taskQueue.EnqueueArgsForCall(1)
			Expect(actualTask).To(Equal(expectedTask))
		})

		It("does not return an error", func() {
			Expect(runErr).NotTo(HaveOccurred())
		})

		Context("when enqueuing the AncestryScanPlan fails", func() {
			BeforeEach(func() {
				taskQueue.EnqueueStub = func(task queue.Task) error {
					switch task.Type() {
					case queue.TaskTypeAncestryScan:
						return errors.New("an-error")
					default:
						return nil
					}
				}
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
			})
		})
	})
})

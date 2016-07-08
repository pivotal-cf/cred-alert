package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("Foreman", func() {
	var (
		foreman *queue.Foreman
	)

	BeforeEach(func() {
		foreman = &queue.Foreman{}
	})

	Describe("building runnable jobs from tasks", func() {
		Context("when the foreman does know how to build the task", func() {
			It("builds the task", func() {
				task := &queuefakes.FakeAckTask{}
				task.TypeReturns("diff-scan")
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"start":      "abc123",
					"end":        "def456"
				}`)

				job, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				diffScan, ok := job.(*queue.DiffScanJob)
				Expect(ok).To(BeTrue())

				Expect(diffScan.Owner).To(Equal("pivotal-cf"))
				Expect(diffScan.Repository).To(Equal("cred-alert"))
				Expect(diffScan.Start).To(Equal("abc123"))
				Expect(diffScan.End).To(Equal("def456"))
			})
		})

		Context("when the foreman doesn't know how to build the task", func() {
			It("returns an error", func() {
				task := &queuefakes.FakeAckTask{}
				task.TypeReturns("unknown-task-type")

				_, err := foreman.BuildJob(task)
				Expect(err).To(MatchError("unknown task type: unknown-task-type"))
			})
		})
	})
})

package queue_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db/dbfakes"
	"cred-alert/github/githubfakes"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"cred-alert/sniff/snifffakes"
)

var _ = Describe("Foreman", func() {
	var (
		foreman queue.Foreman
	)

	BeforeEach(func() {
		foreman = queue.NewForeman(
			&githubfakes.FakeClient{},
			&snifffakes.FakeSniffer{},
			&metricsfakes.FakeEmitter{},
			&notificationsfakes.FakeNotifier{},
			&dbfakes.FakeDiffScanRepository{},
			&dbfakes.FakeCommitRepository{},
			&queuefakes.FakeQueue{},
		)
	})

	Describe("building runnable jobs from tasks", func() {
		Describe("DiffScan Task", func() {
			Context("when the foreman knows how to build the task", func() {
				It("builds the task", func() {
					task := &queuefakes.FakeAckTask{}
					task.TypeReturns(queue.TaskTypeDiffScan)
					task.PayloadReturns(`{
						"owner":      "pivotal-cf",
						"repository": "cred-alert",
						"from":       "abc123",
						"to":         "def456"
					}`)

					job, err := foreman.BuildJob(task)
					Expect(err).NotTo(HaveOccurred())

					diffScan, ok := job.(*queue.DiffScanJob)
					Expect(ok).To(BeTrue())

					Expect(diffScan.Owner).To(Equal("pivotal-cf"))
					Expect(diffScan.Repository).To(Equal("cred-alert"))
					Expect(diffScan.From).To(Equal("abc123"))
					Expect(diffScan.To).To(Equal("def456"))
				})
			})

			Context("payload is not valid json", func() {
				It("returns an error", func() {
					task := &queuefakes.FakeAckTask{}
					task.TypeReturns(queue.TaskTypeDiffScan)
					task.PayloadReturns(`{broken-json":'seriously"}`)

					_, err := foreman.BuildJob(task)
					_, isJsonError := err.(*json.SyntaxError)
					Expect(isJsonError).To(BeTrue())
				})
			})
		})

		Describe("RefScan Task", func() {
			It("builds a ref-scan task", func() {
				task := &queuefakes.FakeAckTask{}
				task.TypeReturns(queue.TaskTypeRefScan)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"ref":        "abc124"
				}`)

				job, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				diffScan, ok := job.(*queue.RefScanJob)
				Expect(ok).To(BeTrue())

				Expect(diffScan.Owner).To(Equal("pivotal-cf"))
				Expect(diffScan.Repository).To(Equal("cred-alert"))
				Expect(diffScan.Ref).To(Equal("abc124"))
			})
		})

		Describe("AncestryScan Task", func() {
			It("builds an ancestry-scan task", func() {
				task := &queuefakes.FakeAckTask{}
				task.TypeReturns(queue.TaskTypeAncestryScan)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"sha":        "abc124",
					"depth": 10
				}`)

				job, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				ancestryScan, ok := job.(*queue.AncestryScanJob)
				Expect(ok).To(BeTrue())

				Expect(ancestryScan.Owner).To(Equal("pivotal-cf"))
				Expect(ancestryScan.Repository).To(Equal("cred-alert"))
				Expect(ancestryScan.SHA).To(Equal("abc124"))
				Expect(ancestryScan.Depth).To(Equal(10))
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

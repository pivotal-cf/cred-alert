package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db/dbfakes"
	"cred-alert/githubclient/githubclientfakes"
	"cred-alert/inflator/inflatorfakes"
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
			&githubclientfakes.FakeClient{},
			&snifffakes.FakeSniffer{},
			&metricsfakes.FakeEmitter{},
			&notificationsfakes.FakeNotifier{},
			&dbfakes.FakeDiffScanRepository{},
			&dbfakes.FakeCredentialRepository{},
			&dbfakes.FakeCommitRepository{},
			&queuefakes.FakeQueue{},
			&inflatorfakes.FakeInflator{},
			&inflatorfakes.FakeScratchSpace{},
		)
	})

	Describe("BuildJob", func() {
		var task *queuefakes.FakeAckTask

		var ItHandlesBrokenJson = func() {
			Context("when the payload is broken json", func() {
				BeforeEach(func() {
					task.PayloadReturns(`{broken-json":'seriously"}`)
				})

				It("returns an error", func() {
					_, err := foreman.BuildJob(task)
					Expect(err).To(HaveOccurred())
				})
			})
		}

		BeforeEach(func() {
			task = &queuefakes.FakeAckTask{}
		})

		Context("when the task is unknown", func() {
			BeforeEach(func() {
				task.TypeReturns("unknown-task-type")
			})

			It("returns an error", func() {
				_, err := foreman.BuildJob(task)
				Expect(err).To(MatchError("unknown task type: unknown-task-type"))
			})
		})

		Context("when the task is a DiffScan", func() {
			BeforeEach(func() {
				task.TypeReturns(queue.TaskTypeDiffScan)
				task.PayloadReturns(`{
						"owner":      "pivotal-cf",
						"repository": "cred-alert",
						"from":       "abc123",
						"to":         "def456"
					}`)
			})

			It("builds the job", func() {
				genericJob, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				job, ok := genericJob.(*queue.DiffScanJob)
				Expect(ok).To(BeTrue())

				Expect(job.Owner).To(Equal("pivotal-cf"))
				Expect(job.Repository).To(Equal("cred-alert"))
				Expect(job.From).To(Equal("abc123"))
				Expect(job.To).To(Equal("def456"))
			})

			ItHandlesBrokenJson()
		})

		Context("when the task is a RefScan", func() {
			BeforeEach(func() {
				task.TypeReturns(queue.TaskTypeRefScan)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"ref":        "abc124"
				}`)
			})

			It("builds the job", func() {
				genericJob, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				job, ok := genericJob.(*queue.RefScanJob)
				Expect(ok).To(BeTrue())

				Expect(job.Owner).To(Equal("pivotal-cf"))
				Expect(job.Repository).To(Equal("cred-alert"))
				Expect(job.Ref).To(Equal("abc124"))
			})

			ItHandlesBrokenJson()
		})

		Context("when the task is a CommitMessageScan", func() {
			BeforeEach(func() {
				task.TypeReturns(queue.TaskTypeCommitMessageScan)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"sha":        "abc124",
					"message":    "commit message",
					"private":    true
				}`)
			})

			It("builds the job", func() {
				genericJob, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				job, ok := genericJob.(*queue.CommitMessageJob)
				Expect(ok).To(BeTrue())

				Expect(job.Owner).To(Equal("pivotal-cf"))
				Expect(job.Repository).To(Equal("cred-alert"))
				Expect(job.SHA).To(Equal("abc124"))
				Expect(job.Message).To(Equal("commit message"))
				Expect(job.Private).To(BeTrue())
			})

			ItHandlesBrokenJson()
		})

		Context("when the task is a AncestryScan", func() {
			BeforeEach(func() {
				task.TypeReturns(queue.TaskTypeAncestryScan)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"sha":        "abc124",
					"depth": 10
				}`)
			})

			It("builds the job", func() {
				genericJob, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				job, ok := genericJob.(*queue.AncestryScanJob)
				Expect(ok).To(BeTrue())

				Expect(job.Owner).To(Equal("pivotal-cf"))
				Expect(job.Repository).To(Equal("cred-alert"))
				Expect(job.SHA).To(Equal("abc124"))
				Expect(job.Depth).To(Equal(10))
			})

			ItHandlesBrokenJson()
		})

		Context("when the task is a PushEvent", func() {
			BeforeEach(func() {
				task.TypeReturns(queue.TaskTypePushEvent)
				task.PayloadReturns(`{
					"owner":      "pivotal-cf",
					"repository": "cred-alert",
					"from":       "from",
					"to":         "to"
				}`)
			})

			It("builds the job", func() {
				genericJob, err := foreman.BuildJob(task)
				Expect(err).NotTo(HaveOccurred())

				job, ok := genericJob.(*queue.PushEventJob)
				Expect(ok).To(BeTrue())

				Expect(job.Owner).To(Equal("pivotal-cf"))
				Expect(job.Repository).To(Equal("cred-alert"))
				Expect(job.From).To(Equal("from"))
				Expect(job.To).To(Equal("to"))
			})

			ItHandlesBrokenJson()
		})
	})
})

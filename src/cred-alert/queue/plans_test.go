package queue_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/queue"
)

var _ = Describe("Plans", func() {
	Describe("PushEventPlan", func() {
		It("can be encoded into a task", func() {
			task := queue.PushEventPlan{
				Owner:      "owner",
				Repository: "repository",
				PushTime:   time.Date(2017, 6, 20, 8, 5, 56, 0, time.UTC),
			}.Task("id-1")

			Expect(task.ID()).To(Equal("id-1"))
			Expect(task.Type()).To(Equal(queue.TaskTypePushEvent))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"pushTime": "2017-06-20T08:05:56Z"
				}`))
		})
	})
})

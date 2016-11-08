package queue_test

import (
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
				Private:    true,
				From:       "sha-1",
				To:         "sha-2",
			}.Task("id-1")

			Expect(task.ID()).To(Equal("id-1"))
			Expect(task.Type()).To(Equal(queue.TaskTypePushEvent))
			Expect(task.Payload()).To(MatchJSON(`
				{
						"owner": "owner",
						"repository": "repository",
						"private": true,
						"from": "sha-1",
						"to": "sha-2"
				}`))
		})
	})
})

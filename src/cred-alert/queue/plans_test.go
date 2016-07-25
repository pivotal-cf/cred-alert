package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/queue"
)

var _ = Describe("Plans", func() {
	Describe("DiffScanPlan", func() {
		It("can be encoded into a task", func() {
			plan := queue.DiffScanPlan{
				Owner:      "owner",
				Repository: "repository",
				Private:    true,
				From:       "abc123",
				To:         "def456",
			}

			task := plan.Task("an-id")
			Expect(task.ID()).To(Equal("an-id"))
			Expect(task.Type()).To(Equal(queue.TaskTypeDiffScan))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"private": true,
					"from": "abc123",
					"to": "def456"
				}
			`))
		})

		It("is a queueable plan", func() {
			var _ queue.Plan = queue.DiffScanPlan{}
		})
	})

	Describe("RefScanPlan", func() {
		It("can be encoded into a task", func() {
			plan := queue.RefScanPlan{
				Owner:      "owner",
				Repository: "repository",
				Private:    true,
				Ref:        "abc123",
			}

			task := plan.Task("an-id")
			Expect(task.ID()).To(Equal("an-id"))
			Expect(task.Type()).To(Equal(queue.TaskTypeRefScan))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"private": true,
					"ref": "abc123"
				}
			`))
		})

		It("is a queueable plan", func() {
			var _ queue.Plan = queue.RefScanPlan{}
		})
	})

	Describe("AncestryScanPlan", func() {
		It("can be encoded into a task", func() {
			plan := queue.AncestryScanPlan{
				Owner:      "owner",
				Repository: "repository",
				Private:    true,
				SHA:        "sha-1",
				Depth:      1,
			}
			task := plan.Task("id-1")
			Expect(task.ID()).To(Equal("id-1"))
			Expect(task.Type()).To(Equal(queue.TaskTypeAncestryScan))
			Expect(task.Payload()).To(MatchJSON(`
				{
						"owner": "owner",
						"repository": "repository",
						"private": true,
						"sha": "sha-1",
						"depth": 1
				}`))
		})

		It("is a queueable plan", func() {
			var _ queue.Plan = queue.AncestryScanPlan{}
		})
	})
})

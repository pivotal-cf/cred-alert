package queue_test

import (
	"fmt"
	"time"

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
				From:       "abc123",
				To:         "def456",
			}

			task := plan.Task("an-id")
			Expect(task.ID()).To(Equal("an-id"))
			Expect(task.Type()).To(Equal("diff-scan"))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
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
				Ref:        "abc123",
			}

			task := plan.Task("an-id")
			Expect(task.ID()).To(Equal("an-id"))
			Expect(task.Type()).To(Equal("ref-scan"))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
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
			expectedTime := time.Now()
			plan := queue.AncestryScanPlan{
				Owner:           "owner",
				Repository:      "repository",
				SHA:             "sha-1",
				CommitTimestamp: expectedTime.Unix(),
				Depth:           1,
			}
			task := plan.Task("id-1")
			Expect(task.ID()).To(Equal("id-1"))
			Expect(task.Type()).To(Equal("ancestry-scan"))
			Expect(task.Payload()).To(MatchJSON(fmt.Sprintf(`
				{
						"owner": "owner",
						"repository": "repository",
						"sha": "sha-1",
						"commit-timestamp": %d,
						"depth": 1
				}`, expectedTime.Unix())))
		})

		It("is a queueable plan", func() {
			var _ queue.Plan = queue.AncestryScanPlan{}
		})
	})
})

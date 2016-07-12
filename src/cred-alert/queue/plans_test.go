package queue_test

import (
	. "cred-alert/queue"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plans", func() {
	Describe("DiffScanPlan", func() {
		It("can be encoded into a task", func() {
			plan := DiffScanPlan{
				Owner:      "owner",
				Repository: "repository",
				Ref:        "refs/head/my-branch",
				From:       "abc123",
				To:         "def456",
			}

			task := plan.Task()
			Expect(task.Type()).To(Equal("diff-scan"))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"ref": "refs/head/my-branch",
					"from": "abc123",
					"to": "def456"
				}
			`))
		})
	})

	Describe("RefScanPlan", func() {
		It("can be encoded into a task", func() {
			plan := RefScanPlan{
				Owner:      "owner",
				Repository: "repository",
				Ref:        "abc123",
			}

			task := plan.Task()
			Expect(task.Type()).To(Equal("ref-scan"))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"ref": "abc123"
				}
			`))
		})
	})
})

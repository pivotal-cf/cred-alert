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
				Start:      "abc123",
				End:        "def456",
			}

			task := plan.Task()
			Expect(task.Type()).To(Equal("diff-scan"))
			Expect(task.Payload()).To(MatchJSON(`
				{
					"owner": "owner",
					"repository": "repository",
					"start": "abc123",
					"end": "def456"
				}
			`))
		})
	})
})

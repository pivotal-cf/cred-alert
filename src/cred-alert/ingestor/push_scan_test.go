package ingestor_test

import (
	"cred-alert/ingestor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-github/github"
)

var _ = Describe("PushScan", func() {
	var (
		event github.PushEvent
	)

	BeforeEach(func() {
		event = github.PushEvent{
			Before: github.String("commit-sha-0"),
			Repo: &github.PushEventRepository{
				Name: github.String("repository-name"),
				Owner: &github.PushEventRepoOwner{
					Name: github.String("repository-owner"),
				},
			},
			Commits: []github.PushEventCommit{
				{ID: github.String("commit-sha-1")},
				{ID: github.String("commit-sha-2")},
				{ID: github.String("commit-sha-3")},
				{ID: github.String("commit-sha-4")},
				{ID: github.String("commit-sha-5")},
			},
		}
	})

	It("can give us the first and last commit of the push", func() {
		scan, valid := ingestor.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.FirstCommit()).To(Equal("commit-sha-0"))
		Expect(scan.LastCommit()).To(Equal("commit-sha-5"))
	})

	It("can extract a value object from a github push event", func() {
		scan, valid := ingestor.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))
		Expect(scan.Diffs).To(Equal([]ingestor.PushScanDiff{
			{Start: "commit-sha-0", End: "commit-sha-1"},
			{Start: "commit-sha-1", End: "commit-sha-2"},
			{Start: "commit-sha-2", End: "commit-sha-3"},
			{Start: "commit-sha-3", End: "commit-sha-4"},
			{Start: "commit-sha-4", End: "commit-sha-5"},
		}))
	})

	It("can have a full repository name", func() {
		scan, valid := ingestor.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))

		Expect(scan.FullRepoName()).To(Equal("repository-owner/repository-name"))
	})

	It("can handle if there are no commits in a push (may not even be possible)", func() {
		event.Commits = []github.PushEventCommit{}

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if there is no before specified", func() {
		event.Before = nil

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if this is the initial push to the repository because the GitHub API doesn't allow this comparison", func() {
		event.Before = github.String("0000000000000000000000000000000000000000")

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})
})

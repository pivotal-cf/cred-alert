package ingestor_test

import (
	"cred-alert/ingestor"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-github/github"
)

var _ = Describe("PushScan", func() {
	var (
		event     github.PushEvent
		fakeTimes [6]time.Time
	)

	BeforeEach(func() {
		now := time.Now()
		for i := 0; i < len(fakeTimes); i++ {
			fakeTimes[i] = now.AddDate(0, 0, i)
		}

		event = github.PushEvent{
			Before: github.String("commit-sha-0"),
			Repo: &github.PushEventRepository{
				Name: github.String("repository-name"),
				Owner: &github.PushEventRepoOwner{
					Name: github.String("repository-owner"),
				},
			},
			Ref: github.String("refs/heads/my-branch"),
			Commits: []github.PushEventCommit{
				{ID: github.String("commit-sha-1"), Timestamp: &github.Timestamp{Time: fakeTimes[1]}},
				{ID: github.String("commit-sha-2"), Timestamp: &github.Timestamp{Time: fakeTimes[2]}},
				{ID: github.String("commit-sha-3"), Timestamp: &github.Timestamp{Time: fakeTimes[3]}},
				{ID: github.String("commit-sha-4"), Timestamp: &github.Timestamp{Time: fakeTimes[4]}},
				{ID: github.String("commit-sha-5"), Timestamp: &github.Timestamp{Time: fakeTimes[5]}},
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
		Expect(scan.Ref).To(Equal("refs/heads/my-branch"))
		Expect(scan.Diffs[0].FromTimestamp).To(Equal(time.Unix(0, 0)))
		Expect(scan.Diffs[0].ToTimestamp).To(Equal(fakeTimes[1]))
		Expect(scan.Diffs[1].FromTimestamp).To(Equal(fakeTimes[1]))
		Expect(scan.Diffs[1].ToTimestamp).To(Equal(fakeTimes[2]))

		Expect(scan.Diffs).To(Equal([]ingestor.PushScanDiff{
			{From: "commit-sha-0", To: "commit-sha-1", FromTimestamp: time.Unix(0, 0), ToTimestamp: fakeTimes[1]},
			{From: "commit-sha-1", To: "commit-sha-2", FromTimestamp: fakeTimes[1], ToTimestamp: fakeTimes[2]},
			{From: "commit-sha-2", To: "commit-sha-3", FromTimestamp: fakeTimes[2], ToTimestamp: fakeTimes[3]},
			{From: "commit-sha-3", To: "commit-sha-4", FromTimestamp: fakeTimes[3], ToTimestamp: fakeTimes[4]},
			{From: "commit-sha-4", To: "commit-sha-5", FromTimestamp: fakeTimes[4], ToTimestamp: fakeTimes[5]},
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
})

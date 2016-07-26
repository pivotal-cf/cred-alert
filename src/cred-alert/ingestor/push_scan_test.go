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
			After:  github.String("commit-sha-5"),
			Before: github.String("commit-sha-0"),
			Repo: &github.PushEventRepository{
				Name: github.String("repository-name"),
				Owner: &github.PushEventRepoOwner{
					Name: github.String("repository-owner"),
				},
			},
		}
	})

	It("can extract a value object from a github push event", func() {
		scan, valid := ingestor.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))
		Expect(scan.From).To(Equal("commit-sha-0"))
		Expect(scan.To).To(Equal("commit-sha-5"))
	})

	It("can have a full repository name", func() {
		scan, valid := ingestor.Extract(event)
		Expect(valid).To(BeTrue())

		Expect(scan.Owner).To(Equal("repository-owner"))
		Expect(scan.Repository).To(Equal("repository-name"))

		Expect(scan.FullRepoName()).To(Equal("repository-owner/repository-name"))
	})

	It("is not valid if there is no before specified", func() {
		event.Before = nil

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if there is no before specified", func() {
		event.After = nil

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})

	It("is not valid if there is no repo specified", func() {
		event.Repo = nil

		_, valid := ingestor.Extract(event)
		Expect(valid).To(BeFalse())
	})
})

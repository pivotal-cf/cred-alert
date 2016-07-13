package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/github/githubfakes"
	"cred-alert/queue"
)

var _ = Describe("RefScan Job", func() {
	var (
		client *githubfakes.FakeClient

		logger *lagertest.TestLogger

		job *queue.RefScanJob
	)

	BeforeEach(func() {
		plan := queue.RefScanPlan{
			Owner:      "repo-owner",
			Repository: "repo-name",
			Ref:        "reference",
		}

		client = &githubfakes.FakeClient{}
		logger = lagertest.NewTestLogger("ref-scan-job")

		job = queue.NewRefScanJob(plan, client)
	})

	It("fetches a link from GitHub", func() {
		err := job.Run(logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(client.ArchiveLinkCallCount()).To(Equal(1))
		lgr, owner, repo := client.ArchiveLinkArgsForCall(0)
		Expect(lgr).To(Equal(logger))
		Expect(owner).To(Equal("repo-owner"))
		Expect(repo).To(Equal("repo-name"))
	})

	It("Downloads the archive", func() {

	})

	It("Unpacks the archive in a temporary directory", func() {

	})
})

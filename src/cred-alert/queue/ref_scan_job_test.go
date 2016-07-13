package queue_test

import (
	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("RefScan Job", func() {
	var (
		archiver *queuefakes.FakeArchiver

		logger *lagertest.TestLogger

		job *queue.RefScanJob
	)

	BeforeEach(func() {
		plan := queue.RefScanPlan{
			Owner:      "repo-owner",
			Repository: "repo-name",
			Ref:        "reference",
		}

		archiver = &queuefakes.FakeArchiver{}
		logger = lagertest.NewTestLogger("ref-scan-job")

		job = queue.NewRefScanJob(plan, archiver)
	})

	It("fetches a link from GitHub", func() {
		err := job.Run(logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(archiver.GetArchiveLinkCallCount()).To(Equal(1))
		archiveOwner, archiveRepo, archiveFormat, archiveOpts := archiver.GetArchiveLinkArgsForCall(0)
		Expect(archiveOwner).To(Equal("repo-owner"))
		Expect(archiveRepo).To(Equal("repo-name"))
		Expect(archiveFormat).To(Equal("tarball"))
		Expect(archiveOpts).To(Equal(&github.RepositoryContentGetOptions{
			Ref: "reference",
		}))
	})
})

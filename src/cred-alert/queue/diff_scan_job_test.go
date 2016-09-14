package queue_test

import (
	"cred-alert/db/dbfakes"
	"cred-alert/githubclient/githubclientfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Diff Scan Job", func() {
	var (
		job                *queue.DiffScanJob
		emitter            *metricsfakes.FakeEmitter
		notifier           *notificationsfakes.FakeNotifier
		fakeGithubClient   *githubclientfakes.FakeClient
		scanRepository     *dbfakes.FakeScanRepository
		diffScanRepository *dbfakes.FakeDiffScanRepository
		plan               queue.DiffScanPlan
		sniffer            *snifffakes.FakeSniffer
		logger             lager.Logger
		credentialCounter  *metricsfakes.FakeCounter

		activeScan *dbfakes.FakeActiveScan
	)

	var owner = "rad-co"
	var repo = "my-awesome-repo"

	var fromGitSha string = "from-git-sha"
	var toGitSha string = "to-git-sha"

	id := "some-id"

	BeforeEach(func() {
		plan = queue.DiffScanPlan{
			Owner:      owner,
			Repository: repo,
			From:       fromGitSha,
			To:         toGitSha,
			Private:    true,
		}
		sniffer = new(snifffakes.FakeSniffer)
		emitter = &metricsfakes.FakeEmitter{}
		notifier = &notificationsfakes.FakeNotifier{}
		fakeGithubClient = new(githubclientfakes.FakeClient)
		scanRepository = &dbfakes.FakeScanRepository{}
		diffScanRepository = &dbfakes.FakeDiffScanRepository{}
		logger = lagertest.NewTestLogger("diff-scan-job-test")

		activeScan = &dbfakes.FakeActiveScan{}
		scanRepository.StartReturns(activeScan)

		credentialCounter = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.violations":
				return credentialCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}
	})

	JustBeforeEach(func() {
		job = queue.NewDiffScanJob(
			fakeGithubClient,
			sniffer,
			emitter,
			notifier,
			diffScanRepository,
			scanRepository,
			plan,
			id,
		)
	})

	It("scans a commit", func() {
		job.Run(logger)

		fakeGithubClient.CompareRefsReturns(nil, errors.New("disaster"))
		Expect(fakeGithubClient.CompareRefsCallCount()).To(Equal(1))
		_, _, _, sha0, sha1 := fakeGithubClient.CompareRefsArgsForCall(0)
		Expect(sha0).To(Equal(fromGitSha))
		Expect(sha1).To(Equal(toGitSha))
	})

	It("saves a record of the diffscan", func() {
		job.Run(logger)
		Expect(diffScanRepository.SaveDiffScanCallCount()).To(Equal(1))

		_, diffScan := diffScanRepository.SaveDiffScanArgsForCall(0)
		Expect(diffScan.Owner).To(Equal(plan.Owner))
		Expect(diffScan.Repository).To(Equal(plan.Repository))
		Expect(diffScan.FromCommit).To(Equal(plan.From))
		Expect(diffScan.ToCommit).To(Equal(plan.To))
		Expect(diffScan.CredentialFound).To(BeFalse())
	})

	Context("when a credential is found", func() {
		var filePath string

		BeforeEach(func() {
			filePath = "some/file/path"

			sniffer.SniffStub = func(logger lager.Logger, scanner sniff.Scanner, handleViolation sniff.ViolationHandlerFunc) error {
				err := handleViolation(logger, scanners.Violation{
					Line: scanners.Line{
						Path:       filePath,
						LineNumber: 6,
						Content:    []byte("other-content"),
					},
				})
				if err != nil {
					return err
				}

				return handleViolation(logger, scanners.Violation{
					Line: scanners.Line{
						Path:       filePath,
						LineNumber: 10,
						Content:    []byte("content"),
					},
				})
			}
		})

		It("emits count of the credentials it has found", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(credentialCounter.IncNCallCount()).To(Equal(1))
			_, amount, tags := credentialCounter.IncNArgsForCall(0)
			Expect(amount).To(Equal(2))
			Expect(tags).To(ConsistOf("private"))
		})

		It("registers a credential", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(scanRepository.StartCallCount()).To(Equal(1))
			_, typee, _, _ := scanRepository.StartArgsForCall(0)
			Expect(typee).To(Equal("diff-scan"))

			Expect(activeScan.RecordCredentialCallCount()).To(Equal(2))
			Expect(activeScan.FinishCallCount()).To(Equal(1))

			credential := activeScan.RecordCredentialArgsForCall(0)
			Expect(credential.Owner).To(Equal(plan.Owner))
			Expect(credential.Repository).To(Equal(plan.Repository))
			Expect(credential.SHA).To(Equal(toGitSha))
			Expect(credential.Path).To(Equal("some/file/path"))
			Expect(credential.LineNumber).To(Equal(6))

			credential = activeScan.RecordCredentialArgsForCall(1)
			Expect(credential.Owner).To(Equal(plan.Owner))
			Expect(credential.Repository).To(Equal(plan.Repository))
			Expect(credential.SHA).To(Equal(toGitSha))
			Expect(credential.Path).To(Equal("some/file/path"))
			Expect(credential.LineNumber).To(Equal(10))
		})

		It("sends a notification", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(notifier.SendBatchNotificationCallCount()).To(Equal(1))

			_, notifications := notifier.SendBatchNotificationArgsForCall(0)
			Expect(notifications).To(HaveLen(2))

			Expect(notifications[0].Owner).To(Equal(plan.Owner))
			Expect(notifications[0].Repository).To(Equal(plan.Repository))
			Expect(notifications[0].SHA).To(Equal(toGitSha))
			Expect(notifications[0].Path).To(Equal("some/file/path"))
			Expect(notifications[0].LineNumber).To(Equal(6))
			Expect(notifications[0].Private).To(Equal(plan.Private))

			Expect(notifications[1].Owner).To(Equal(plan.Owner))
			Expect(notifications[1].Repository).To(Equal(plan.Repository))
			Expect(notifications[1].SHA).To(Equal(toGitSha))
			Expect(notifications[1].Path).To(Equal("some/file/path"))
			Expect(notifications[1].LineNumber).To(Equal(10))
			Expect(notifications[1].Private).To(Equal(plan.Private))
		})

		It("saves a record of the diffscan and that credentials were found", func() {
			job.Run(logger)
			Expect(diffScanRepository.SaveDiffScanCallCount()).To(Equal(1))

			_, diffScan := diffScanRepository.SaveDiffScanArgsForCall(0)
			Expect(diffScan.Owner).To(Equal(plan.Owner))
			Expect(diffScan.Repository).To(Equal(plan.Repository))
			Expect(diffScan.FromCommit).To(Equal(plan.From))
			Expect(diffScan.ToCommit).To(Equal(plan.To))
			Expect(diffScan.CredentialFound).To(BeTrue())
		})

		Context("when the notification fails to send", func() {
			BeforeEach(func() {
				notifier.SendBatchNotificationReturns(errors.New("disaster"))
			})

			It("fails the job", func() {
				err := job.Run(logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when scanning a public repo", func() {
			BeforeEach(func() {
				plan.Private = false
			})

			It("It emits count with the public tag", func() {
				err := job.Run(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(credentialCounter.IncNCallCount()).To(Equal(1))
				_, amount, tags := credentialCounter.IncNArgsForCall(0)
				Expect(amount).Should(Equal(2))
				Expect(tags).To(ConsistOf("public"))
			})

			It("sends a notification with private set to false", func() {
				job.Run(logger)

				Expect(notifier.SendBatchNotificationCallCount()).To(Equal(1))
				_, notifications := notifier.SendBatchNotificationArgsForCall(0)

				for _, notification := range notifications {
					Expect(notification.Private).To(Equal(plan.Private))
				}
			})
		})
	})

	Context("when the diffScanRepository returns an error", func() {
		BeforeEach(func() {
			diffScanRepository.SaveDiffScanReturns(errors.New("Disaster"))
		})

		It("fails the job", func() {
			err := job.Run(logger)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when we fail to fetch the diff", func() {
		BeforeEach(func() {
			fakeGithubClient.CompareRefsReturns(nil, errors.New("disaster"))
		})

		It("does not try to scan the diff", func() {
			err := job.Run(logger)
			Expect(err).To(HaveOccurred())

			Expect(sniffer.SniffCallCount()).To(BeZero())
			Expect(credentialCounter.IncCallCount()).To(BeZero())
		})
	})
})

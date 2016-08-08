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
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Diff Scan Job", func() {
	var (
		job                *queue.DiffScanJob
		emitter            *metricsfakes.FakeEmitter
		notifier           *notificationsfakes.FakeNotifier
		fakeGithubClient   *githubclientfakes.FakeClient
		diffScanRepository *dbfakes.FakeDiffScanRepository
		plan               queue.DiffScanPlan
		sniffer            *snifffakes.FakeSniffer
		logger             lager.Logger
		credentialCounter  *metricsfakes.FakeCounter
	)

	var owner = "rad-co"
	var repo = "my-awesome-repo"
	var repoFullName = fmt.Sprintf("%s/%s", owner, repo)

	var fromGitSha string = "from-git-sha"
	var toGitSha string = "to-git-sha"
	var lineNumber = 1

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
		diffScanRepository = &dbfakes.FakeDiffScanRepository{}
		logger = lagertest.NewTestLogger("diff-scan-job-test")

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
			plan,
			id,
		)
	})

	It("scans a commit", func() {
		job.Run(logger)

		fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))
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
				return handleViolation(logger, scanners.Line{
					Path:       filePath,
					LineNumber: 1,
					Content:    []byte("content"),
				})
			}
		})

		It("emits count of the credentials it has found", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(credentialCounter.IncCallCount()).To(Equal(1))
			_, tags := credentialCounter.IncArgsForCall(0)
			Expect(tags).To(HaveLen(1))
			Expect(tags).To(ConsistOf("private"))
		})

		It("sends a notification", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(notifier.SendNotificationCallCount()).To(Equal(1))

			_, repository, sha, line, private := notifier.SendNotificationArgsForCall(0)

			Expect(repository).To(Equal(repoFullName))
			Expect(sha).To(Equal(toGitSha))
			Expect(line).To(Equal(scanners.Line{
				Path:       "some/file/path",
				LineNumber: lineNumber,
				Content:    []byte("content"),
			}))
			Expect(private).To(Equal(plan.Private))
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

		It("logs the violation", func() {
			err := job.Run(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("handle-violation"))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"line-number":%d`, lineNumber)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, owner)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"path":"%s"`, filePath)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"from":"%s"`, fromGitSha)))
			Expect(logger).To(gbytes.Say(`"private":true`))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, repo)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"task-id":"%s"`, id)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"to":"%s"`, toGitSha)))
		})

		Context("when the notification fails to send", func() {
			BeforeEach(func() {
				notifier.SendNotificationReturns(errors.New("disaster"))
			})

			It("fails the job", func() {
				err := job.Run(logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When scanning a public repo", func() {
			BeforeEach(func() {
				plan.Private = false
			})

			It("It emits count with the public tag", func() {
				err := job.Run(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(credentialCounter.IncCallCount()).To(Equal(1))
				_, tags := credentialCounter.IncArgsForCall(0)
				Expect(tags).To(HaveLen(1))
				Expect(tags).To(ConsistOf("public"))
				Expect(logger).To(gbytes.Say(`"private":false`))
			})

			It("sends a notification with private set to false", func() {
				job.Run(logger)

				Expect(notifier.SendNotificationCallCount()).To(Equal(1))
				_, _, _, _, private := notifier.SendNotificationArgsForCall(0)
				Expect(private).To(Equal(plan.Private))
			})
		})
	})

	Context("When the diffScanRepository returns an error", func() {
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
			fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))
		})

		It("does not try to scan the diff", func() {
			err := job.Run(logger)
			Expect(err).To(HaveOccurred())

			Expect(sniffer.SniffCallCount()).To(BeZero())
			Expect(credentialCounter.IncCallCount()).To(BeZero())
		})
	})
})

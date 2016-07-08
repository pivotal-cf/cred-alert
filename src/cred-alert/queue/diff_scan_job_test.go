package queue_test

import (
	"cred-alert/github/githubfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/sniff"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Diff Scan Job", func() {
	var (
		job               *queue.DiffScanJob
		emitter           *metricsfakes.FakeEmitter
		notifier          *notificationsfakes.FakeNotifier
		fakeGithubClient  *githubfakes.FakeClient
		plan              queue.DiffScanPlan
		sniffFunc         func(lager.Logger, sniff.Scanner, func(sniff.Line))
		logger            lager.Logger
		credentialCounter *metricsfakes.FakeCounter
	)
	var owner string = "rad-co"
	var repo = "my-awesome-repo"
	var repoFullName = fmt.Sprintf("rad-co/%s", repo)

	var start string = "before"
	var end string = "after"

	BeforeEach(func() {
		plan = queue.DiffScanPlan{
			Owner:      owner,
			Repository: repo,
			Start:      start,
			End:        end,
		}
		sniffFunc = func(lager.Logger, sniff.Scanner, func(sniff.Line)) {}
		emitter = &metricsfakes.FakeEmitter{}
		notifier = &notificationsfakes.FakeNotifier{}
		fakeGithubClient = new(githubfakes.FakeClient)
		logger = lagertest.NewTestLogger("diff-scan-job")

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
			sniffFunc,
			emitter,
			notifier,
			plan,
		)
	})

	It("scans a commit", func() {
		job.Run(logger)

		fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))
		Expect(fakeGithubClient.CompareRefsCallCount()).To(Equal(1))
		_, _, _, sha0, sha1 := fakeGithubClient.CompareRefsArgsForCall(0)
		Expect(sha0).To(Equal(start))
		Expect(sha1).To(Equal(end))
	})

	Context("when a credential is found", func() {
		var filePath string

		BeforeEach(func() {
			filePath = "some/file/path"

			sniffFunc = func(logger lager.Logger, scanner sniff.Scanner, handleViolation func(sniff.Line)) {
				handleViolation(sniff.Line{
					Path:       filePath,
					LineNumber: 1,
					Content:    "content",
				})
			}
		})

		It("emits count of the credentials it has found", func() {
			job.Run(logger)

			Expect(credentialCounter.IncCallCount()).To(Equal(1))
		})

		It("sends a notification", func() {
			job.Run(logger)

			Expect(notifier.SendNotificationCallCount()).To(Equal(1))

			_, repository, sha, line := notifier.SendNotificationArgsForCall(0)

			Expect(repository).To(Equal(repoFullName))
			Expect(sha).To(Equal(end))
			Expect(line).To(Equal(sniff.Line{
				Path:       "some/file/path",
				LineNumber: 1,
				Content:    "content",
			}))
		})
	})

	// Context("when we fail to fetch the diff", func() {
	// 	var wasScanned bool

	// 	BeforeEach(func() {
	// 		wasScanned = false

	// 		fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))

	// 		sniffFunc = func(lager.Logger, sniff.Scanner, func(sniff.Line)) {
	// 			wasScanned = true
	// 		}
	// 	})

	// 	It("does not try to scan the diff", func() {
	// 		eventHandler.HandleEvent(logger, event)

	// 		Expect(wasScanned).To(BeFalse())
	// 		Expect(credentialCounter.IncNCallCount()).To(Equal(0))
	// 	})
	// })
})

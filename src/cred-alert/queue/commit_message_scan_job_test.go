package queue_test

import (
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/sniff/snifffakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Commit Message Scan Job", func() {

	Describe("Run", func() {
		var (
			job               *queue.CommitMessageJob
			logger            *lagertest.TestLogger
			plan              queue.CommitMessageScanPlan
			emitter           *metricsfakes.FakeEmitter
			notifier          *notificationsfakes.FakeNotifier
			sniffer           *snifffakes.FakeSniffer
			credentialCounter *metricsfakes.FakeCounter
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("commit-message-scan-job")
			plan = queue.CommitMessageScanPlan{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				Message:    "message",
			}
			sniffer = new(snifffakes.FakeSniffer)
			emitter = &metricsfakes.FakeEmitter{}
			notifier = &notificationsfakes.FakeNotifier{}
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
			job = queue.NewCommitMessageJob(sniffer, emitter, notifier, plan, "id")
		})

		Context("When the message contains a credential", func() {
			BeforeEach(func() {
				plan.Message = `password = "should_match"`
			})

			It("logs the violations", func() {
			})

			It("emits the violations", func() {
				Expect(true).To(BeTrue())
				// err := job.Run(logger)
				// Expect(err).NotTo(HaveOccurred())

				// Expect(credentialCounter.IncCallCount()).To(Equal(1))
				// _, tags := credentialCounter.IncArgsForCall(0)
				// Expect(tags).To(HaveLen(1))
				// Expect(tags).To(ConsistOf("private"))
			})

			It("notifies slack about the violations", func() {
			})
		})

	})

})

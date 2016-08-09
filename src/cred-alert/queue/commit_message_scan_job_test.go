package queue_test

import (
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/queue"
	"cred-alert/scanners"
	"cred-alert/sniff"
	"cred-alert/sniff/snifffakes"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
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
				Private:    true,
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
			job = queue.NewCommitMessageJob(sniffer, emitter, notifier, plan)
		})

		It("logs basic info", func() {
			err := job.Run(logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(logger).To(gbytes.Say("scan-commit-message"))
			Expect(logger).To(gbytes.Say(plan.Owner))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"private":%v`, plan.Private)))
			Expect(logger).To(gbytes.Say(plan.Repository))
			Expect(logger).To(gbytes.Say(plan.SHA))
		})

		Context("When the message contains a credential", func() {
			violatingLine := scanners.Line{
				LineNumber: 999,
			}

			BeforeEach(func() {
				sniffer.SniffStub = func(logger lager.Logger, scanner sniff.Scanner, handleViolation sniff.ViolationHandlerFunc) error {
					return handleViolation(logger, violatingLine)
				}
			})

			It("logs the violation", func() {
				err := job.Run(logger)

				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("handle-violation"))
			})

			It("emits the violations", func() {
				job.Run(logger)

				Expect(credentialCounter.IncCallCount()).To(Equal(1))
				_, tags := credentialCounter.IncArgsForCall(0)
				Expect(tags).To(ContainElement("private"))
				Expect(tags).To(ContainElement("commit-message"))
			})

			It("notifies slack about the violations", func() {
				job.Run(logger)

				Expect(notifier.SendNotificationCallCount()).To(Equal(1))

				_, notification := notifier.SendNotificationArgsForCall(0)
				Expect(notification.Owner).To(Equal(plan.Owner))
				Expect(notification.Repository).To(Equal(plan.Repository))
				Expect(notification.SHA).To(Equal(plan.SHA))
				Expect(notification.Path).To(Equal(violatingLine.Path))
				Expect(notification.LineNumber).To(Equal(violatingLine.LineNumber))
				Expect(notification.Private).To(Equal(plan.Private))
			})

			Context("when the repo is public", func() {
				BeforeEach(func() {
					plan.Private = false
				})

				It("log, emits, notifies the public-ness", func() {
					job.Run(logger)

					Expect(logger).To(gbytes.Say(fmt.Sprintf(`"private":%v`, false)))

					Expect(credentialCounter.IncCallCount()).To(Equal(1))
					_, tags := credentialCounter.IncArgsForCall(0)
					Expect(tags).To(ContainElement("public"))

					_, notification := notifier.SendNotificationArgsForCall(0)
					Expect(notification.Private).To(Equal(plan.Private))
				})
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

		})

	})

})

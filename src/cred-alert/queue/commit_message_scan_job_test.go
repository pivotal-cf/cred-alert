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

	"github.com/pivotal-golang/lager"
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
			taskId            string
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
			taskId = "task-id"
		})

		JustBeforeEach(func() {
			job = queue.NewCommitMessageJob(sniffer, emitter, notifier, plan, taskId)
		})

		It("logs basic info", func() {
			err := job.Run(logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(logger).To(gbytes.Say("scan-commit-message"))
			Expect(logger).To(gbytes.Say(plan.Owner))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"private":%v`, plan.Private)))
			Expect(logger).To(gbytes.Say(plan.Repository))
			Expect(logger).To(gbytes.Say(plan.SHA))
			Expect(logger).To(gbytes.Say(taskId))
		})

		Context("When the message contains a credential", func() {
			violatingLine := scanners.Line{
				LineNumber: 999,
			}

			BeforeEach(func() {
				sniffer.SniffStub = func(logger lager.Logger, scanner sniff.Scanner, handleViolation func(scanners.Line) error) error {
					return handleViolation(violatingLine)
				}
			})

			It("logs the violation", func() {
				err := job.Run(logger)

				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("found-credentials"))
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

				_, repository, sha, line, private := notifier.SendNotificationArgsForCall(0)

				Expect(repository).To(Equal(plan.Owner))
				Expect(sha).To(Equal(plan.SHA))
				Expect(line).To(Equal(violatingLine))
				Expect(private).To(Equal(plan.Private))
			})

			Context("When the repo is public", func() {
				BeforeEach(func() {
					plan.Private = false
				})

				It("log, emits, notifies the public-ness", func() {
					job.Run(logger)

					Expect(logger).To(gbytes.Say(fmt.Sprintf(`"private":%v`, false)))

					Expect(credentialCounter.IncCallCount()).To(Equal(1))
					_, tags := credentialCounter.IncArgsForCall(0)
					Expect(tags).To(ContainElement("public"))

					_, _, _, _, notificationPrivacy := notifier.SendNotificationArgsForCall(0)
					Expect(notificationPrivacy).To(Equal(false))
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

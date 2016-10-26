package queue_test

import (
	"cred-alert/db/dbfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	"cloud.google.com/go/pubsub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RetryHandler", func() {
	var (
		logger            *lagertest.TestLogger
		processor         *queuefakes.FakePubSubProcessor
		failedMessageRepo *dbfakes.FakeFailedMessageRepository
		acker             *queuefakes.FakeAcker

		msg *pubsub.Message

		retryHandler queue.RetryHandler
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("retry-handler")
		failedMessageRepo = &dbfakes.FakeFailedMessageRepository{}

		msg = &pubsub.Message{
			ID: "my-special-message-id",
		}

		acker = &queuefakes.FakeAcker{}
		processor = &queuefakes.FakePubSubProcessor{}

		retryHandler = queue.NewRetryHandler(failedMessageRepo, processor, acker)
	})

	JustBeforeEach(func() {
		retryHandler.ProcessMessage(logger, msg)
	})

	Context("when the message can be processed", func() {
		BeforeEach(func() {
			processor.ProcessReturns(false, nil)
		})

		It("does not store the failed message", func() {
			Expect(failedMessageRepo.RegisterFailedMessageCallCount()).Should(Equal(0))
		})

		It("does not put the message back on the queue", func() {
			Expect(acker.AckCallCount()).To(Equal(1))

			msg, ack := acker.AckArgsForCall(0)
			Expect(msg.ID).To(Equal("my-special-message-id"))
			Expect(ack).To(BeTrue())
		})

		It("removes the message from the failed messages repository", func() {
			Expect(failedMessageRepo.RemoveFailedMessageCallCount()).To(Equal(1))

			passedLogger, messageId := failedMessageRepo.RemoveFailedMessageArgsForCall(0)
			Expect(passedLogger).To(BeIdenticalTo(logger))
			Expect(messageId).To(Equal("my-special-message-id"))
		})
	})

	Context("when the message cannot be processed", func() {
		var err error

		BeforeEach(func() {
			err = errors.New("My Special Error")
		})

		Context("when the message is valid", func() {
			BeforeEach(func() {
				processor.ProcessReturns(true, err)
			})

			It("stores the failed message", func() {
				Expect(failedMessageRepo.RegisterFailedMessageCallCount()).To(Equal(1))

				_, messageId := failedMessageRepo.RegisterFailedMessageArgsForCall(0)
				Expect(messageId).To(Equal("my-special-message-id"))
			})

			Context("when storing the message fails", func() {
				BeforeEach(func() {
					failedMessageRepo.RegisterFailedMessageReturns(0, errors.New("some-error"))
				})

				It("logs failure", func() {
					Expect(logger).To(gbytes.Say(`retry-handler.failed-to-save-msg`))
				})
			})

			It("does not remove the message from the failed messages repository", func() {
				Expect(failedMessageRepo.RemoveFailedMessageCallCount()).To(Equal(0))
			})

			It("passes the logger down so that context can be stored", func() {
				Expect(failedMessageRepo.RegisterFailedMessageCallCount()).To(Equal(1))

				passedLogger, _ := failedMessageRepo.RegisterFailedMessageArgsForCall(0)

				Expect(passedLogger).To(BeIdenticalTo(logger))
			})

			Context("when the message has been retried enough times", func() {
				BeforeEach(func() {
					failedMessageRepo.RegisterFailedMessageReturns(3, nil)
				})

				It("does not put the messages back on the queue", func() {
					Expect(acker.AckCallCount()).To(Equal(1))

					msg, ack := acker.AckArgsForCall(0)
					Expect(msg.ID).To(Equal("my-special-message-id"))
					Expect(ack).To(BeTrue())
				})

				It("marks message as dead", func() {
					Expect(failedMessageRepo.MarkFailedMessageAsDeadCallCount()).To(Equal(1))

					passedLogger, messageId := failedMessageRepo.MarkFailedMessageAsDeadArgsForCall(0)
					Expect(passedLogger).To(BeIdenticalTo(logger))
					Expect(messageId).To(Equal("my-special-message-id"))
				})
			})

			Context("when the message has not been retried enough times yet", func() {
				BeforeEach(func() {
					failedMessageRepo.RegisterFailedMessageReturns(2, nil)
				})

				It("puts the messages back on the queue", func() {
					Expect(acker.AckCallCount()).To(Equal(1))

					msg, ack := acker.AckArgsForCall(0)
					Expect(msg.ID).To(Equal("my-special-message-id"))
					Expect(ack).To(BeFalse())
				})

				It("does not mark message as dead", func() {
					Expect(failedMessageRepo.MarkFailedMessageAsDeadCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the message is invalid", func() {
			BeforeEach(func() {
				processor.ProcessReturns(false, err)
			})

			It("does not store the failed message", func() {
				Expect(failedMessageRepo.RegisterFailedMessageCallCount()).To(Equal(0))
			})

			It("does not remove the message from the failed messages repository", func() {
				Expect(failedMessageRepo.RemoveFailedMessageCallCount()).To(Equal(0))
			})

			It("does not put the messages back on the queue", func() {
				Expect(acker.AckCallCount()).To(Equal(1))

				msg, ack := acker.AckArgsForCall(0)
				Expect(msg.ID).To(Equal("my-special-message-id"))
				Expect(ack).To(BeTrue())
			})
		})
	})
})

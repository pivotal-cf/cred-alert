package queue_test

import (
	"cred-alert/crypto/cryptofakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/revok/revokfakes"
	"errors"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushEventProcessor", func() {
	var (
		logger              *lagertest.TestLogger
		pushEventProcessor  queue.PubSubProcessor
		changeFetcher       *revokfakes.FakeChangeFetcher
		verifier            *cryptofakes.FakeVerifier
		message             *pubsub.Message
		emitter             *metricsfakes.FakeEmitter
		verifyFailedCounter *metricsfakes.FakeCounter
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("ingestor")
		changeFetcher = &revokfakes.FakeChangeFetcher{}
		verifier = &cryptofakes.FakeVerifier{}
		verifyFailedCounter = &metricsfakes.FakeCounter{}
		emitter = &metricsfakes.FakeEmitter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "queue.push_event_processor.verify.failed":
				return verifyFailedCounter
			default:
				return &metricsfakes.FakeCounter{}
			}
		}

		pushEventProcessor = queue.NewPushEventProcessor(changeFetcher, verifier, emitter)

	})

	It("verifies the signature", func() {
		message = &pubsub.Message{
			Attributes: map[string]string{
				"signature": "c29tZS1zaWduYXR1cmU=",
			},
			Data: []byte("some-message"),
		}

		pushEventProcessor.Process(logger, message)

		Expect(verifier.VerifyCallCount()).To(Equal(1))
		message, signature := verifier.VerifyArgsForCall(0)
		Expect(message).To(Equal([]byte("some-message")))
		Expect(signature).To(Equal([]byte("some-signature")))
	})

	Context("when the signature fails to decode", func() {
		BeforeEach(func() {
			message = &pubsub.Message{
				Attributes: map[string]string{
					"signature": "Undecodable Signature",
				},
			}
		})

		It("returns an error", func() {
			retriable, err := pushEventProcessor.Process(logger, message)

			Expect(retriable).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("base64"))
		})
	})

	Context("when the signature is invalid", func() {
		var err error

		BeforeEach(func() {
			message = &pubsub.Message{
				Attributes: map[string]string{
					"signature": "InvalidSignature",
				},
			}

			err = errors.New("invalid signature")

			verifier.VerifyReturns(err)
		})

		It("returns an error", func() {
			retriable, err := pushEventProcessor.Process(logger, message)
			Expect(retriable).To(BeFalse())
			Expect(err).To(Equal(err))
		})

		It("increments the error counter", func() {
			pushEventProcessor.Process(logger, message)

			Expect(verifyFailedCounter.IncCallCount()).To(Equal(1))
		})
	})

	Context("when the payload is a valid JSON PushEventPlan", func() {
		BeforeEach(func() {
			task := queue.PushEventPlan{
				Owner:      "some-owner",
				Repository: "some-repo",
				From:       "from-sha",
				To:         "to-sha",
				Private:    true,
			}.Task("message-id")

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   task.ID(),
					"type": task.Type(),
				},
				Data: []byte(task.Payload()),
			}
		})

		It("does not increment the verifyFailedCounter", func() {
			pushEventProcessor.Process(logger, message)
			Expect(verifyFailedCounter.IncCallCount()).To(Equal(0))
		})

		It("tries to do a fetch", func() {
			pushEventProcessor.Process(logger, message)
			Expect(changeFetcher.FetchCallCount()).To(Equal(1))
			_, actualOwner, actualName, actualReenable := changeFetcher.FetchArgsForCall(0)
			Expect(actualOwner).To(Equal("some-owner"))
			Expect(actualName).To(Equal("some-repo"))
			Expect(actualReenable).To(BeTrue())
		})

		Context("when the fetch succeeds", func() {
			BeforeEach(func() {
				changeFetcher.FetchReturns(nil)
			})

			It("does not retry or return an error", func() {
				retry, err := pushEventProcessor.Process(logger, message)
				Expect(retry).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the fetch fails", func() {
			BeforeEach(func() {
				changeFetcher.FetchReturns(errors.New("an-error"))
			})

			It("returns an error that can be retried", func() {
				retry, err := pushEventProcessor.Process(logger, message)
				Expect(retry).To(BeTrue())
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when the payload is not valid JSON", func() {
		BeforeEach(func() {
			bs := []byte("some bad bytes")

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(logger, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an error that cannot be retried", func() {
			retry, err := pushEventProcessor.Process(logger, message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the payload is a valid JSON for a PushEventPlan but is missing the repository", func() {
		BeforeEach(func() {
			bs := []byte(`{
				"owner":"some-owner"
			}`)

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(logger, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(logger, message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the payload is a valid JSON for a PushEventPlan but is missing the owner", func() {
		BeforeEach(func() {
			bs := []byte(`{
				"repository":"some-repository"
			}`)

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(logger, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(logger, message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})
})

package queue_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/crypto/cryptofakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("Signature Checker", func() {
	var (
		ctx      context.Context
		child    *queuefakes.FakePubSubProcessor
		verifier *cryptofakes.FakeVerifier
		message  *pubsub.Message

		emitter             *metricsfakes.FakeEmitter
		verifyFailedCounter *metricsfakes.FakeCounter

		checker queue.PubSubProcessor
	)

	BeforeEach(func() {
		ctx = lagerctx.NewContext(context.Background(), lagertest.NewTestLogger("signature-check"))
		verifier = &cryptofakes.FakeVerifier{}
		child = &queuefakes.FakePubSubProcessor{}
		verifyFailedCounter = &metricsfakes.FakeCounter{}

		emitter = &metricsfakes.FakeEmitter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "queue.verification_failures":
				return verifyFailedCounter
			}

			panic("unexpected metric: " + name)
		}

		checker = queue.NewSignatureCheck(verifier, emitter, child)
	})

	Context("when the signature is correct", func() {
		It("passes the message through to the child processor", func() {
			message = &pubsub.Message{
				Attributes: map[string]string{
					"signature": "c29tZS1zaWduYXR1cmU=",
				},
				Data: []byte("some-message"),
			}

			_, err := checker.Process(ctx, message)
			Expect(err).NotTo(HaveOccurred())

			Expect(child.ProcessCallCount()).To(Equal(1))
		})
	})

	Context("when the signature fails to decode", func() {
		BeforeEach(func() {
			message = &pubsub.Message{
				Attributes: map[string]string{
					"signature": "Undecodable Signature",
				},
			}
		})

		It("returns an error and does not call the child", func() {
			retry, err := checker.Process(ctx, message)
			Expect(err).To(MatchError(ContainSubstring("base64")))
			Expect(retry).To(BeFalse())

			Expect(child.ProcessCallCount()).Should(BeZero())
		})

		It("increments the failure counter", func() {
			_, err := checker.Process(ctx, message)
			Expect(err).To(HaveOccurred())

			Expect(verifyFailedCounter.IncCallCount()).To(Equal(1))
		})
	})

	Context("when the signature is invalid", func() {
		BeforeEach(func() {
			message = &pubsub.Message{
				Attributes: map[string]string{
					"signature": "InvalidSignature",
				},
			}

			verifier.VerifyReturns(errors.New("bad signature!"))
		})

		It("returns an error and does not call the child", func() {
			retry, err := checker.Process(ctx, message)
			Expect(err).To(HaveOccurred())
			Expect(retry).To(BeFalse())

			Expect(child.ProcessCallCount()).Should(BeZero())
		})

		It("increments the failure counter", func() {
			_, err := checker.Process(ctx, message)
			Expect(err).To(HaveOccurred())

			Expect(verifyFailedCounter.IncCallCount()).To(Equal(1))
		})
	})
})

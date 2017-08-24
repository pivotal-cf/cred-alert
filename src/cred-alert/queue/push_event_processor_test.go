package queue_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"golang.org/x/net/context"

	"cred-alert/lgctx"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("PushEventProcessor", func() {
	var (
		ctx           context.Context
		fakeClock     *fakeclock.FakeClock
		changeFetcher *queuefakes.FakeChangeFetcher
		message       *pubsub.Message

		emitter       *metricsfakes.FakeEmitter
		endToEndGauge *metricsfakes.FakeGauge

		pushEventProcessor queue.PubSubProcessor
	)

	BeforeEach(func() {
		ctx = lgctx.NewContext(context.Background(), lagertest.NewTestLogger("ingestor"))
		changeFetcher = &queuefakes.FakeChangeFetcher{}
		endToEndGauge = &metricsfakes.FakeGauge{}

		emitter = &metricsfakes.FakeEmitter{}
		emitter.GaugeStub = func(name string) metrics.Gauge {
			switch name {
			case "queue.end-to-end.duration":
				return endToEndGauge
			}
			return nil
		}

		now := time.Date(2017, 10, 8, 16, 20, 42, 0, time.UTC)

		fakeClock = fakeclock.NewFakeClock(now)

		pushEventProcessor = queue.NewPushEventProcessor(changeFetcher, emitter, fakeClock, nil)
	})

	Context("when the payload is a valid JSON PushEventPlan", func() {
		BeforeEach(func() {
			task := queue.PushEventPlan{
				Owner:      "some-owner",
				Repository: "some-repo",
				PushTime:   time.Date(2017, 10, 8, 16, 19, 22, 0, time.UTC),
			}.Task("message-id")

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   task.ID(),
					"type": task.Type(),
				},
				Data: []byte(task.Payload()),
			}
		})

		It("tries to do a fetch", func() {
			pushEventProcessor.Process(ctx, message)
			Expect(changeFetcher.FetchCallCount()).To(Equal(1))
			_, _, actualOwner, actualName, actualReenable := changeFetcher.FetchArgsForCall(0)
			Expect(actualOwner).To(Equal("some-owner"))
			Expect(actualName).To(Equal("some-repo"))
			Expect(actualReenable).To(BeTrue())
		})

		Context("when the fetch succeeds", func() {
			BeforeEach(func() {
				changeFetcher.FetchReturns(nil)
			})

			It("does not retry or return an error", func() {
				retry, err := pushEventProcessor.Process(ctx, message)
				Expect(retry).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})

			It("emits the total processing time", func() {
				_, err := pushEventProcessor.Process(ctx, message)
				Expect(err).NotTo(HaveOccurred())

				Expect(endToEndGauge.UpdateCallCount()).To(Equal(1))

				passedLogger, duration, _ := endToEndGauge.UpdateArgsForCall(0)
				Expect(passedLogger).NotTo(BeNil())
				Expect(duration).To(Equal(float32(80)))
			})
		})

		Context("when the fetch fails", func() {
			BeforeEach(func() {
				changeFetcher.FetchReturns(errors.New("an-error"))
			})

			It("returns an error that can be retried", func() {
				retry, err := pushEventProcessor.Process(ctx, message)
				Expect(retry).To(BeTrue())
				Expect(err).To(HaveOccurred())
			})

			It("does not emit the processing time", func() {
				_, err := pushEventProcessor.Process(ctx, message)
				Expect(err).To(HaveOccurred())

				Expect(endToEndGauge.UpdateCallCount()).To(BeZero())
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
			pushEventProcessor.Process(ctx, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an error that cannot be retried", func() {
			retry, err := pushEventProcessor.Process(ctx, message)
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
			pushEventProcessor.Process(ctx, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(ctx, message)
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
			pushEventProcessor.Process(ctx, message)
			Expect(changeFetcher.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(ctx, message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})
})

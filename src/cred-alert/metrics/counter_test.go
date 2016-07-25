package metrics_test

import (
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Counters", func() {
	var (
		metric  *metricsfakes.FakeMetric
		counter metrics.Counter
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		metric = &metricsfakes.FakeMetric{}
		logger = lagertest.NewTestLogger("counter")
	})

	JustBeforeEach(func() {
		counter = metrics.NewCounter(metric)
	})

	It("does not emit anything if the count is zero", func() {
		counter.IncN(logger, 0)

		Expect(metric.UpdateCallCount()).To(Equal(0))
	})

	It("can increment once", func() {
		counter.Inc(logger, "tag1", "tag2")

		Expect(metric.UpdateCallCount()).To(Equal(1))
		callLogger, callValue, tags := metric.UpdateArgsForCall(0)
		Expect(callLogger).To(Equal(logger))
		Expect(callValue).To(Equal(float32(1)))
		Expect(tags).To(ConsistOf("tag1", "tag2"))
	})

	It("can increment many times", func() {
		counter.IncN(logger, 2, "tag1", "tag2")

		Expect(metric.UpdateCallCount()).To(Equal(1))
		callLogger, callValue, tags := metric.UpdateArgsForCall(0)
		Expect(callLogger).To(Equal(logger))
		Expect(callValue).To(Equal(float32(2)))
		Expect(tags).To(ConsistOf("tag1", "tag2"))
	})

	Context("nullCounter", func() {
		JustBeforeEach(func() {
			counter = metrics.NewNullCounter(metric)
		})

		It("calls update when Inc is called", func() {
			counter.Inc(logger, "tag1", "tag2")

			Expect(metric.UpdateCallCount()).To(Equal(1))
			callLogger, callValue, tags := metric.UpdateArgsForCall(0)
			Expect(callLogger).To(Equal(logger))
			Expect(callValue).To(Equal(float32(1)))
			Expect(len(tags)).To(Equal(2))
			Expect(tags).To(ConsistOf("tag1", "tag2"))
		})

		It("calls update when IncN is called", func() {
			passedValue := 3
			counter.IncN(logger, passedValue, "tag1", "tag2")

			Expect(metric.UpdateCallCount()).To(Equal(1))
			callLogger, callValue, tags := metric.UpdateArgsForCall(0)
			Expect(callLogger).To(Equal(logger))
			Expect(callValue).To(Equal(float32(passedValue)))
			Expect(len(tags)).To(Equal(2))
			Expect(tags).To(ConsistOf("tag1", "tag2"))
		})
	})
})

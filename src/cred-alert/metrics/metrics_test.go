package metrics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/datadog"
	"cred-alert/datadog/datadogfakes"
	"cred-alert/metrics"
)

var _ = Describe("Metrics", func() {
	var (
		logger *lagertest.TestLogger

		client  *datadogfakes.FakeClient
		emitter metrics.Emitter
	)

	environment := "test"

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("metrics")

		client = &datadogfakes.FakeClient{}
		emitter = metrics.NewEmitter(client, environment)
	})

	Describe("counters", func() {
		It("does not emit anything if the count is zero", func() {
			counter := emitter.Counter("counter")

			counter.IncN(logger, 0)

			Expect(client.BuildMetricCallCount()).Should(Equal(0))
			Expect(client.PublishSeriesCallCount()).Should(Equal(0))
		})

		It("can increment once", func() {
			counter := emitter.Counter("counter")

			counter.Inc(logger)

			Expect(client.BuildMetricCallCount()).Should(Equal(1))

			counterType, counterName, counterCount, counterTag := client.BuildMetricArgsForCall(0)
			Expect(counterType).To(Equal(datadog.COUNTER_METRIC_TYPE))
			Expect(counterName).To(Equal("counter"))
			Expect(counterCount).To(Equal(float32(1)))
			Expect(counterTag).To(Equal([]string{environment}))

			expectedMetric := datadog.Metric{}
			client.BuildMetricReturns(expectedMetric)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesArgsForCall(0)).To(ConsistOf(expectedMetric))
		})

		It("can increment many times", func() {
			counter := emitter.Counter("counter")

			counter.IncN(logger, 234)

			counterType, counterName, counterCount, counterTag := client.BuildMetricArgsForCall(0)
			Expect(counterType).To(Equal(datadog.COUNTER_METRIC_TYPE))
			Expect(counterName).To(Equal("counter"))
			Expect(counterCount).To(Equal(float32(234)))
			Expect(counterTag).To(Equal([]string{environment}))

			expectedMetric := datadog.Metric{}
			client.BuildMetricReturns(expectedMetric)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesArgsForCall(0)).To(ConsistOf(expectedMetric))
		})
	})

	Describe("guages", func() {
		It("does emit zero values", func() {
			guage := emitter.Guage("myGuage")

			guage.Update(logger, 123)

			Expect(client.BuildMetricCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
		})

		It("Updates a metric value", func() {
			guage := emitter.Guage("myGuage")

			guage.Update(logger, 234)

			counterType, counterName, counterCount, counterTag := client.BuildMetricArgsForCall(0)
			Expect(counterType).To(Equal(datadog.GUAGE_METRIC_TYPE))
			Expect(counterName).To(Equal("myGuage"))
			Expect(counterCount).To(Equal(float32(234)))
			Expect(counterTag).To(Equal([]string{environment}))

			expectedMetric := datadog.Metric{}
			client.BuildMetricReturns(expectedMetric)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesArgsForCall(0)).To(ConsistOf(expectedMetric))
		})

	})
})

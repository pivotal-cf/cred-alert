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

			Expect(client.BuildCountMetricCallCount()).Should(Equal(0))
			Expect(client.PublishSeriesCallCount()).Should(Equal(0))
		})

		It("can increment once", func() {
			counter := emitter.Counter("counter")

			counter.Inc(logger)

			Expect(client.BuildCountMetricCallCount()).Should(Equal(1))

			counterName, counterCount, counterTag := client.BuildCountMetricArgsForCall(0)
			Expect(counterName).To(Equal("counter"))
			Expect(counterCount).To(Equal(float32(1)))
			Expect(counterTag).To(Equal([]string{environment}))

			expectedMetric := datadog.Metric{}
			client.BuildCountMetricReturns(expectedMetric)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesArgsForCall(0)).To(ConsistOf(expectedMetric))
		})

		It("can increment many times", func() {
			counter := emitter.Counter("counter")

			counter.IncN(logger, 234)

			counterName, counterCount, counterTag := client.BuildCountMetricArgsForCall(0)
			Expect(counterName).To(Equal("counter"))
			Expect(counterCount).To(Equal(float32(234)))
			Expect(counterTag).To(Equal([]string{environment}))

			expectedMetric := datadog.Metric{}
			client.BuildCountMetricReturns(expectedMetric)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			Expect(client.PublishSeriesArgsForCall(0)).To(ConsistOf(expectedMetric))
		})
	})
})

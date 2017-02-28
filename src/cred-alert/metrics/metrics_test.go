package metrics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/datadog"
	"cred-alert/datadog/datadogfakes"
	"cred-alert/metrics"
)

var _ = Describe("Metrics", func() {
	var (
		logger *lagertest.TestLogger

		client             *datadogfakes.FakeClient
		metric             metrics.Gauge
		expectedMetricType string
		expectedMetricName string
		expectedEnv        string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("metrics")

		client = &datadogfakes.FakeClient{}
		expectedEnv = "env"
		emitter := metrics.NewEmitter(client, expectedEnv)
		expectedMetricType = "name"
		expectedMetricName = "type"
		metric = metrics.NewMetric(expectedMetricName, expectedMetricType, emitter)
	})

	It("calls BuildMetric and PublishSeries", func() {
		expectedValue := float32(0)
		metric.Update(logger, expectedValue)

		expectedMetric := datadog.Metric{}
		client.BuildMetricReturns(expectedMetric)

		Expect(client.BuildMetricCallCount()).To(Equal(1))

		metricType, name, value, env := client.BuildMetricArgsForCall(0)
		Expect(metricType).To(Equal(expectedMetricType))
		Expect(name).To(Equal(expectedMetricName))
		Expect(value).To(Equal(expectedValue))
		Expect(env[0]).To(Equal(expectedEnv))

		Eventually(client.PublishSeriesCallCount).Should(Equal(1))

		_, actualMetric := client.PublishSeriesArgsForCall(0)
		Expect(actualMetric).To(ContainElement(expectedMetric))
	})

	It("can have tags", func() {
		metric.Update(logger, 3.52, "name:value", "othername:othervalue")

		Expect(client.BuildMetricCallCount()).Should(Equal(1))
		_, _, _, tags := client.BuildMetricArgsForCall(0)
		Expect(tags).To(ConsistOf(expectedEnv, "name:value", "othername:othervalue"))
	})
})

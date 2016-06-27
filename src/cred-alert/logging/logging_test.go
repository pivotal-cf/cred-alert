package logging_test

import (
	"cred-alert/datadog/datadogfakes"
	"cred-alert/logging"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Logging", func() {
	var (
		logger *lagertest.TestLogger

		client  *datadogfakes.FakeClient
		emitter logging.Emitter
	)

	environment := "test"

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("logging")

		client = &datadogfakes.FakeClient{}
		emitter = logging.NewEmitter(client, environment)
	})

	Context("CountViolation", func() {
		It("sends a violation count to datadog", func() {
			emitter.CountViolation(logger, 1)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			series := client.PublishSeriesArgsForCall(0)

			metric := series[0]

			Expect(metric.Name).To(Equal("cred_alert.violations"))
			Expect(metric.Type).To(Equal("count"))
			Expect(metric.Tags).To(ConsistOf(environment))

			point := metric.Points[0]

			Expect(point.Timestamp).To(BeTemporally("~", time.Now()))
			Expect(point.Value).To(BeNumerically("==", 1))
		})

		It("can increment by more than one", func() {
			emitter.CountViolation(logger, 9)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			series := client.PublishSeriesArgsForCall(0)

			metric := series[0]

			Expect(metric.Name).To(Equal("cred_alert.violations"))
			Expect(metric.Type).To(Equal("count"))
			Expect(metric.Tags).To(ConsistOf(environment))

			point := metric.Points[0]

			Expect(point.Timestamp).To(BeTemporally("~", time.Now()))
			Expect(point.Value).To(BeNumerically("==", 9))
		})
	})

	Context("CountAPIRequest", func() {
		It("sends a github API request event to datadog", func() {
			emitter.CountAPIRequest(logger)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			series := client.PublishSeriesArgsForCall(0)

			metric := series[0]

			Expect(metric.Name).To(Equal("cred_alert.webhook_requests"))
			Expect(metric.Type).To(Equal("count"))
			Expect(metric.Tags).To(ConsistOf(environment))

			point := metric.Points[0]

			Expect(point.Timestamp).To(BeTemporally("~", time.Now()))
			Expect(point.Value).To(BeNumerically("==", 1))
		})
	})
})

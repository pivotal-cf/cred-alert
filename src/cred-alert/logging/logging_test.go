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

	Describe("counters", func() {
		It("can increment once", func() {
			counter := emitter.Counter("counter")

			counter.Inc(logger)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			series := client.PublishSeriesArgsForCall(0)

			metric := series[0]

			Expect(metric.Name).To(Equal("counter"))
			Expect(metric.Type).To(Equal("count"))
			Expect(metric.Tags).To(ConsistOf(environment))

			point := metric.Points[0]

			Expect(point.Timestamp).To(BeTemporally("~", time.Now()))
			Expect(point.Value).To(BeNumerically("==", 1))
		})

		It("does not emit anything if the count is zero", func() {
			counter := emitter.Counter("counter")

			counter.IncN(logger, 0)

			Expect(client.PublishSeriesCallCount()).Should(Equal(0))
		})

		It("can increment many times", func() {
			counter := emitter.Counter("counter")

			counter.IncN(logger, 234)

			Expect(client.PublishSeriesCallCount()).Should(Equal(1))
			series := client.PublishSeriesArgsForCall(0)

			metric := series[0]

			Expect(metric.Name).To(Equal("counter"))
			Expect(metric.Type).To(Equal("count"))
			Expect(metric.Tags).To(ConsistOf(environment))

			point := metric.Points[0]

			Expect(point.Timestamp).To(BeTemporally("~", time.Now()))
			Expect(point.Value).To(BeNumerically("==", 234))
		})
	})
})

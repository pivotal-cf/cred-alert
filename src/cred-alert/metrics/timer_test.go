package metrics_test

import (
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"

	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timer", func() {
	var (
		timer  metrics.Timer
		metric *metricsfakes.FakeMetric
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		metric = &metricsfakes.FakeMetric{}
		logger = lagertest.NewTestLogger("timer")
		timer = metrics.NewTimer(metric)
	})

	It("handles a closure", func() {
		hasBeenCalled := false
		timer.Time(logger, func() {
			hasBeenCalled = true
		}, "tag:value")

		Expect(hasBeenCalled).To(BeTrue())
		Expect(metric.UpdateCallCount()).To(Equal(1))
		_, dur, tags := metric.UpdateArgsForCall(0)

		Expect(dur).To(BeNumerically(">", 0))
		Expect(tags).To(ConsistOf("tag:value"))
		Expect(logger.LogMessages()).To(HaveLen(1))
	})
})

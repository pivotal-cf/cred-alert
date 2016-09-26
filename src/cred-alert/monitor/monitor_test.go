package monitor_test

import (
	"errors"
	"os"
	"time"

	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"

	"cred-alert/metrics/metricsfakes"
	"cred-alert/monitor"
	"cred-alert/monitor/monitorfakes"
)

var _ = Describe("Monitor", func() {
	var (
		process ifrit.Process

		logger   *lagertest.TestLogger
		ghClient *monitorfakes.FakeRateClient
		clock    *fakeclock.FakeClock
		emitter  *metricsfakes.FakeEmitter

		interval time.Duration
		gauge    *metricsfakes.FakeGauge
	)

	BeforeEach(func() {
		interval = 60 * time.Second

		logger = lagertest.NewTestLogger("monitor")
		ghClient = &monitorfakes.FakeRateClient{}
		clock = fakeclock.NewFakeClock(time.Now())
		emitter = &metricsfakes.FakeEmitter{}
		gauge = &metricsfakes.FakeGauge{}
		emitter.GaugeReturns(gauge)

		runner := monitor.NewMonitor(logger, ghClient, emitter, clock, interval)
		process = ifrit.Background(runner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
	})

	Context("after the process has just started", func() {
		It("has not asked GitHub for the remaining requests", func() {
			Consistently(ghClient.RateLimitsCallCount).Should(BeZero())
		})

		It("has not sent anything", func() {
			Consistently(gauge.UpdateCallCount).Should(BeZero())
		})
	})

	Context("after the process has been running for one interval", func() {
		BeforeEach(func() {
			ghClient.RateLimitsReturns(rateLimit(772), nil, nil)
			clock.WaitForNWatchersAndIncrement(interval, 1)
		})

		It("monitors the current value of the requests remaining", func() {
			Eventually(gauge.UpdateCallCount).Should(Equal(1))
			_, remainingRequests, _ := gauge.UpdateArgsForCall(0)
			Expect(remainingRequests).To(BeNumerically("==", 772))
		})
	})

	Context("after the process has been running for many intervals", func() {
		BeforeEach(func() {
			ghClient.RateLimitsReturns(rateLimit(72), nil, nil)

			clock.WaitForNWatchersAndIncrement(interval, 1)
		})

		It("keeps monitoring the current value of the requests remaining", func() {
			Eventually(gauge.UpdateCallCount).Should(Equal(1))
			_, remainingRequests, _ := gauge.UpdateArgsForCall(0)
			Expect(remainingRequests).To(BeNumerically("==", 72))

			clock.WaitForNWatchersAndIncrement(interval, 1)

			Eventually(gauge.UpdateCallCount).Should(Equal(2))
			_, remainingRequests, _ = gauge.UpdateArgsForCall(1)
			Expect(remainingRequests).To(BeNumerically("==", 72))
		})
	})

	Context("if getting the rate from github fails", func() {
		BeforeEach(func() {
			ghClient.RateLimitsReturns(nil, nil, errors.New("some-error"))
			clock.WaitForNWatchersAndIncrement(interval, 1)
		})

		It("does not exit", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		It("logs an error message", func() {
			Eventually(logger).Should(gbytes.Say("some-error"))
		})
	})
})

func rateLimit(limit int) *github.RateLimits {
	return &github.RateLimits{
		Core: &github.Rate{
			Remaining: limit,
		},
	}
}

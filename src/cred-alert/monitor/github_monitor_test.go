package monitor_test

import (
	"cred-alert/metrics/metricsfakes"
	"cred-alert/monitor"
	"cred-alert/monitor/monitorfakes"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GithubMonitor", func() {
	var (
		process       ifrit.Process
		logger        *lagertest.TestLogger
		clock         *fakeclock.FakeClock
		emitter       *metricsfakes.FakeEmitter
		ghServiceFake *monitorfakes.FakeGithubService

		interval   time.Duration
		gauge      *metricsfakes.FakeGauge
		server     *ghttp.Server
		response   string
		statusCode int
	)

	BeforeEach(func() {
		interval = 60 * time.Second

		logger = lagertest.NewTestLogger("GithubMonitor")
		ghServiceFake = &monitorfakes.FakeGithubService{}
		clock = fakeclock.NewFakeClock(time.Now())
		emitter = &metricsfakes.FakeEmitter{}
		gauge = &metricsfakes.FakeGauge{}
		emitter.GaugeReturns(gauge)

		runner := monitor.NewGithubMonitor(
			logger,
			ghServiceFake,
			clock,
			interval,
			emitter,
		)
		process = ifrit.Background(runner)

		server = ghttp.NewServer()

		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/"),
			ghttp.RespondWith(statusCode, response),
		))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
	})

	Context("after the process has just started", func() {
		It("has not sent anything", func() {
			Consistently(gauge.UpdateCallCount).Should(BeZero())
		})
	})

	Context("after the process has been running for one interval", func() {
		BeforeEach(func() {
			clock.WaitForNWatchersAndIncrement(interval, 1)
		})

		It("checks the current status of github", func() {
			Eventually(gauge.UpdateCallCount).Should(Equal(1))
			_, status, _ := gauge.UpdateArgsForCall(0)
			Expect(ghServiceFake.StatusCallCount()).To(Equal(1))
			Expect(status).To(BeZero())
		})
	})

	Context("if getting the status fails", func() {
		BeforeEach(func() {
			ghServiceFake.StatusReturns(1, errors.New("some-error"))
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

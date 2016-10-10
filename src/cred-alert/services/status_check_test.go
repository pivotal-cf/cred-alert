package services_test

import (
	"cred-alert/metrics/metricsfakes"
	"cred-alert/services"
	"cred-alert/services/servicesfakes"
	"errors"
	"fmt"
	"net/http"
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

var _ = Describe("StatusCheck", func() {
	var (
		process   ifrit.Process
		logger    *lagertest.TestLogger
		clock     *fakeclock.FakeClock
		emitter   *metricsfakes.FakeEmitter
		ghService *servicesfakes.FakeGithubServiceClient

		interval   time.Duration
		gauge      *metricsfakes.FakeGauge
		server     *ghttp.Server
		response   string
		statusCode int
	)

	BeforeEach(func() {
		interval = 60 * time.Second

		logger = lagertest.NewTestLogger("StatusCheck")
		ghService = &servicesfakes.FakeGithubServiceClient{}
		clock = fakeclock.NewFakeClock(time.Now())
		emitter = &metricsfakes.FakeEmitter{}
		gauge = &metricsfakes.FakeGauge{}
		emitter.GaugeReturns(gauge)

		runner := services.NewGithubService(logger, ghService, emitter, clock, interval)
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
			Expect(status).To(BeZero())
		})
	})

	Context("if getting the status fails", func() {
		BeforeEach(func() {
			ghService.GithubStatusReturns(1, errors.New("some-error"))
			clock.WaitForNWatchersAndIncrement(interval, 1)
		})

		It("does not exit", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		FIt("logs an error message", func() {
			Eventually(logger).Should(gbytes.Say("some-error"))
		})
	})
})

var _ = Describe("GithubStatus", func() {

	var (
		server     *ghttp.Server
		response   string
		statusCode int
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/"),
			ghttp.RespondWith(statusCode, response),
		))
	})

	Context("when request fails", func() {
		BeforeEach(func() {
			statusCode = http.StatusBadRequest
		})

		It("returns StatusBadRequest", func() {
			status, err := services.GithubStatus(server.URL())
			Expect(status).To(Equal(1))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when request succeeds", func() {
		BeforeEach(func() {
			statusCode = http.StatusOK
		})

		Context("and github is not ok", func() {
			BeforeEach(func() {
				response = `{"status":"foo","last_updated":"2016-10-07T21:12:08Z"}`
			})

			It("returns 1", func() {
				Expect(services.GithubStatus(server.URL())).To(Equal(1))
			})
		})

		Context("and github returns status ok", func() {
			BeforeEach(func() {
				response = `{"status":"good","last_updated":"2016-10-07T21:12:08Z"}`
			})

			It("returns 0", func() {
				Expect(services.GithubStatus(server.URL())).To(Equal(0))
			})
		})
	})
})

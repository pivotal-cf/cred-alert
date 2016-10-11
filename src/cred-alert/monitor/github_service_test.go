package monitor_test

import (
	"cred-alert/monitor"
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("GithubService", func() {
	Describe("Status", func() {
		var (
			server     *ghttp.Server
			response   string
			statusCode int
			ghService  monitor.GithubService
			logger     *lagertest.TestLogger
		)

		BeforeEach(func() {
			server = ghttp.NewServer()
			logger = lagertest.NewTestLogger("GithubService")
			ghService = monitor.NewGithubService(logger)
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
				status, err := ghService.Status(server.URL())
				Expect(status).To(Equal(1))
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("github-response-error"))
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
					Expect(ghService.Status(server.URL())).To(Equal(1))
				})
			})

			Context("and github returns status ok", func() {
				BeforeEach(func() {
					response = `{"status":"good","last_updated":"2016-10-07T21:12:08Z"}`
				})

				It("returns 0", func() {
					Expect(ghService.Status(server.URL())).To(Equal(0))
				})
			})
		})
	})
})

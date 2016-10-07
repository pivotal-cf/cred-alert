package queue_test

import (
	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HttpEnqueuer", func() {
	var (
		logger   *lagertest.TestLogger
		enqueuer queue.Enqueuer
		server   *ghttp.Server
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		logger = lagertest.NewTestLogger("http-enqueuer")
		enqueuer = queue.NewHTTPEnqueuer(logger, server.URL()+"/endpoint")
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Enqueue", func() {
		var (
			task *queuefakes.FakeTask
		)

		BeforeEach(func() {
			task = &queuefakes.FakeTask{}
			task.PayloadReturns("some-payload")

			server.AppendHandlers(
				ghttp.VerifyRequest("POST", "/endpoint", ""),
				ghttp.VerifyHeader(http.Header{"Content-Type": []string{"application/json"}}),
				ghttp.VerifyBody([]byte(task.Payload())),
				ghttp.RespondWith(http.StatusAccepted, "", nil),
			)
		})

		It("posts the task payload to the endpoint", func() {
			err := enqueuer.Enqueue(task)
			Expect(err).NotTo(HaveOccurred())

			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})
	})
})

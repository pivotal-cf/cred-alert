package webhook_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/webhook"
)

var _ = Describe("Webhook", func() {
	var (
		logger *lagertest.TestLogger

		handler http.Handler
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("webhook")
		handler = webhook.Handler(logger, "example-key")
	})

	It("Responds with 200", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{}"))
		fakeRequest.Header.Set("X-Hub-Signature", "sha1=aca19cdfbae3091d5977eb8b00e95451f1e94571")

		handler.ServeHTTP(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(200))
	})

	It("Respons with 403 when the signature is invalid", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{}"))
		fakeRequest.Header.Set("X-Hub-Signature", "thisaintnohmacsignature")

		handler.ServeHTTP(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(403))
	})

	It("Responds with 400 when the payload is not valid JSON", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{'ooops:---"))
		fakeRequest.Header.Set("X-Hub-Signature", "sha1=77812823a4bf1dae951267bbbb7b7f737cf418c6")

		handler.ServeHTTP(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(400))
	})
})

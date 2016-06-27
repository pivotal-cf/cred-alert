package webhook_test

import (
	. "cred-alert/webhook"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webhook", func() {
	var webhookHandler WebhookHandler
	var secretKey string

	BeforeSuite(func() {
		secretKey = "example-key"
	})

	BeforeEach(func() {
		webhookHandler = NewWebhookHandler(secretKey, "")
	})

	It("Responds with 200", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{}"))
		fakeRequest.Header.Set("X-Hub-Signature", "sha1=aca19cdfbae3091d5977eb8b00e95451f1e94571")

		webhookHandler.HandleWebhook(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(200))
	})

	It("Responds with 403 when the signature is invalid", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{}"))
		fakeRequest.Header.Set("X-Hub-Signature", "thisaintnohmacsignature")

		webhookHandler.HandleWebhook(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(403))
	})

	It("Responds with 400 when the payload is not valid JSON", func() {
		fakeWriter := httptest.NewRecorder()
		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", strings.NewReader("{'ooops:---"))
		fakeRequest.Header.Set("X-Hub-Signature", "sha1=77812823a4bf1dae951267bbbb7b7f737cf418c6")

		webhookHandler.HandleWebhook(fakeWriter, fakeRequest)

		Expect(fakeWriter.Code).To(Equal(400))
	})

})

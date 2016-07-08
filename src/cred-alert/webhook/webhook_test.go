package webhook_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/queue/queuefakes"
	"cred-alert/webhook"
	"cred-alert/webhook/webhookfakes"
)

var _ = Describe("Webhook", func() {
	var (
		logger *lagertest.TestLogger

		handler      http.Handler
		eventHandler *webhookfakes.FakeEventHandler

		fakeRequest *http.Request
		recorder    *httptest.ResponseRecorder

		token     string
		fakeQueue *queuefakes.FakeQueue
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("webhook")
		recorder = httptest.NewRecorder()
		eventHandler = &webhookfakes.FakeEventHandler{}
		fakeQueue = &queuefakes.FakeQueue{}
		token = "example-key"

		handler = webhook.Handler(logger, eventHandler, token, fakeQueue)
	})

	pushEvent := github.PushEvent{
		Before: github.String("beef04"),
		After:  github.String("af7e40"),
	}

	Context("when the request is properly formed", func() {
		BeforeEach(func() {
			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(pushEvent)
			Expect(err).NotTo(HaveOccurred())

			macHeader := fmt.Sprintf("sha1=%s", messageMAC(token, body.Bytes()))

			fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			fakeRequest.Header.Set("X-Hub-Signature", macHeader)
		})

		It("responds with 200", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("handles and scans the event directly", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Eventually(eventHandler.HandleEventCallCount).Should(Equal(1))
		})
	})

	Context("when the signature is invalid", func() {
		BeforeEach(func() {
			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(pushEvent)
			Expect(err).NotTo(HaveOccurred())

			fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			fakeRequest.Header.Set("X-Hub-Signature", "thisaintnohmacsignature")
		})

		It("responds with 403", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(recorder.Code).To(Equal(http.StatusForbidden))
		})

		It("does not directly handle the event", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Consistently(eventHandler.HandleEventCallCount).Should(BeZero())
		})

		It("does not enqueue any tasks", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(fakeQueue.EnqueueCallCount()).To(BeZero())
		})
	})

	Context("when the payload is not valid JSON", func() {
		BeforeEach(func() {
			badJSON := bytes.NewBufferString("{'ooops:---")

			fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", badJSON)
			fakeRequest.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", messageMAC(token, badJSON.Bytes())))
		})

		It("responds with 400", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("does not directly handle the event", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Consistently(eventHandler.HandleEventCallCount).Should(BeZero())
		})

		It("does not enqueue any tasks", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(fakeQueue.EnqueueCallCount()).To(BeZero())
		})
	})
})

func messageMAC(key string, body []byte) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write(body)
	return fmt.Sprintf("%x", mac.Sum(nil))
}

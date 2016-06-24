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

	"cred-alert/webhook"
	"cred-alert/webhook/webhookfakes"
)

var _ = Describe("Webhook", func() {
	var (
		logger *lagertest.TestLogger

		handler http.Handler
		scanner *webhookfakes.FakeScanner

		recorder *httptest.ResponseRecorder

		token string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("webhook")
		recorder = httptest.NewRecorder()
		scanner = &webhookfakes.FakeScanner{}
		token = "example-key"

		handler = webhook.Handler(logger, scanner, token)
	})

	It("scans the push event successfully", func() {
		pushEvent := github.PushEvent{
			Before: github.String("beef04"),
			After:  github.String("af7e40"),
		}

		body := &bytes.Buffer{}
		err := json.NewEncoder(body).Encode(pushEvent)
		Expect(err).NotTo(HaveOccurred())

		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", body)
		fakeRequest.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", messageMAC(token, body.Bytes())))

		handler.ServeHTTP(recorder, fakeRequest)

		Eventually(scanner.ScanPushEventCallCount).Should(Equal(1))
		Expect(recorder.Code).To(Equal(http.StatusOK))
	})

	It("responds with 403 when the signature is invalid", func() {
		pushEvent := github.PushEvent{
			Before: github.String("beef04"),
			After:  github.String("af7e40"),
		}

		body := &bytes.Buffer{}
		err := json.NewEncoder(body).Encode(pushEvent)
		Expect(err).NotTo(HaveOccurred())

		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", body)
		fakeRequest.Header.Set("X-Hub-Signature", "thisaintnohmacsignature")

		handler.ServeHTTP(recorder, fakeRequest)

		Consistently(scanner.ScanPushEventCallCount).Should(BeZero())
		Expect(recorder.Code).To(Equal(http.StatusForbidden))
	})

	It("responds with 400 when the payload is not valid JSON", func() {
		badJSON := bytes.NewBufferString("{'ooops:---")

		fakeRequest, _ := http.NewRequest("POST", "http://example.com/webhook", badJSON)
		fakeRequest.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", messageMAC(token, badJSON.Bytes())))

		handler.ServeHTTP(recorder, fakeRequest)

		Consistently(scanner.ScanPushEventCallCount).Should(BeZero())
		Expect(recorder.Code).To(Equal(http.StatusBadRequest))
	})
})

func messageMAC(key string, body []byte) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write(body)
	return fmt.Sprintf("%x", mac.Sum(nil))
}

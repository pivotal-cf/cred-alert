package ingestor_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/ingestor"
	"cred-alert/ingestor/ingestorfakes"
)

var _ = Describe("Webhook", func() {
	var (
		logger *lagertest.TestLogger

		handler http.Handler
		in      *ingestorfakes.FakeIngestor

		fakeRequest *http.Request
		recorder    *httptest.ResponseRecorder

		token     string
		fakeTimes [6]time.Time
	)

	BeforeEach(func() {
		for i := 0; i < len(fakeTimes); i++ {
			fakeTimes[i] = time.Now().AddDate(0, 0, i)
		}
		logger = lagertest.NewTestLogger("ingestor")
		recorder = httptest.NewRecorder()
		in = &ingestorfakes.FakeIngestor{}
		token = "example-key"

		handler = ingestor.Handler(logger, in, token)
	})

	pushEvent := github.PushEvent{
		Before: github.String("abc123bef04e"),
		After:  github.String("def456af4e4"),
		Repo: &github.PushEventRepository{
			Private:  github.Bool(true),
			Name:     github.String("repository-name"),
			FullName: github.String("repository-owner/repository-name"),
			Owner: &github.PushEventRepoOwner{
				Name: github.String("repository-owner"),
			},
		},
		Ref: github.String("refs/heads/my-branch"),
		Commits: []github.PushEventCommit{
			{ID: github.String("commit-sha-1"), Timestamp: &github.Timestamp{Time: fakeTimes[1]}},
			{ID: github.String("commit-sha-2"), Timestamp: &github.Timestamp{Time: fakeTimes[2]}},
			{ID: github.String("commit-sha-3"), Timestamp: &github.Timestamp{Time: fakeTimes[3]}},
			{ID: github.String("commit-sha-4"), Timestamp: &github.Timestamp{Time: fakeTimes[4]}},
			{ID: github.String("commit-sha-5"), Timestamp: &github.Timestamp{Time: fakeTimes[5]}},
		},
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

			Eventually(in.IngestPushScanCallCount).Should(Equal(1))
		})

		Context("when we fail to ingest the message", func() {
			BeforeEach(func() {
				in.IngestPushScanReturns(errors.New("disaster"))
			})

			It("returns a 500", func() {
				handler.ServeHTTP(recorder, fakeRequest)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
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

			Expect(in.IngestPushScanCallCount()).To(BeZero())
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

			Expect(in.IngestPushScanCallCount()).To(BeZero())
		})
	})
})

func messageMAC(key string, body []byte) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write(body)
	return fmt.Sprintf("%x", mac.Sum(nil))
}

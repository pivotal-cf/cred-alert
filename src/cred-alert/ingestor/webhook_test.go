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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/google/go-github/github"

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

		configuredTokens []string
		signingToken     string
		pushEvent        github.PushEvent
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("ingestor")
		recorder = httptest.NewRecorder()
		in = &ingestorfakes.FakeIngestor{}
		configuredTokens = []string{"example-key"}
		signingToken = configuredTokens[0]
		pushEvent = github.PushEvent{
			Before: github.String("commit-sha-0"),
			After:  github.String("commit-sha-5"),
			Repo: &github.PushEventRepository{
				Private:  github.Bool(true),
				Name:     github.String("repository-name"),
				FullName: github.String("repository-owner/repository-name"),
				Owner: &github.PushEventRepoOwner{
					Name: github.String("repository-owner"),
				},
			},
			Commits: []github.PushEventCommit{
				{ID: github.String("commit-sha-1")},
				{ID: github.String("commit-sha-2")},
				{ID: github.String("commit-sha-3")},
				{ID: github.String("commit-sha-4")},
				{ID: github.String("commit-sha-5")},
			},
		}

		handler = ingestor.NewHandler(logger, in, configuredTokens)
	})

	Context("when the request is properly formed", func() {
		JustBeforeEach(func() {
			handler = ingestor.NewHandler(logger, in, configuredTokens)

			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(pushEvent)
			Expect(err).NotTo(HaveOccurred())

			macHeader := fmt.Sprintf("sha1=%s", messageMAC(signingToken, body.Bytes()))

			fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			fakeRequest.Header.Set("X-Hub-Signature", macHeader)
			fakeRequest.Header.Set("X-GitHub-Delivery", "delivery-id")
		})

		It("responds with 200", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("handles and scans the event directly", func() {
			handler.ServeHTTP(recorder, fakeRequest)

			Eventually(in.IngestPushScanCallCount).Should(Equal(1))
			_, actualScan, actualGitHubID := in.IngestPushScanArgsForCall(0)

			Expect(actualScan).To(Equal(ingestor.PushScan{
				Owner:      "repository-owner",
				Repository: "repository-name",
				From:       "commit-sha-0",
				To:         "commit-sha-5",
				Private:    true,
			}))
			Expect(actualGitHubID).To(Equal("delivery-id"))
		})

		Context("when multiple configuredTokens are configured", func() {
			BeforeEach(func() {
				configuredTokens = []string{"example-token-a", "example-token-b"}
			})

			Context("when the request is signed with the first token", func() {
				BeforeEach(func() {
					signingToken = "example-token-a"
				})

				It("responds with 200", func() {
					handler.ServeHTTP(recorder, fakeRequest)
					Expect(recorder.Code).To(Equal(http.StatusOK))
				})
			})

			Context("when the request is signed with the second token", func() {
				BeforeEach(func() {
					signingToken = "example-token-b"
				})

				It("responds with 200", func() {
					handler.ServeHTTP(recorder, fakeRequest)
					Expect(recorder.Code).To(Equal(http.StatusOK))
				})
			})
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

		Context("when the payload is missing a Before", func() {
			BeforeEach(func() {
				pushEvent.Before = nil
			})

			It("responds with OK", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("does not enqueue anything", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(in.IngestPushScanCallCount()).To(BeZero())
			})
		})

		Context("when the payload is missing an After", func() {
			BeforeEach(func() {
				pushEvent.After = nil
			})

			It("responds with OK", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("does not enqueue anything", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(in.IngestPushScanCallCount()).To(BeZero())
			})
		})
	})

	Context("when the signature is invalid", func() {
		JustBeforeEach(func() {
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
		JustBeforeEach(func() {
			badJSON := bytes.NewBufferString("{'ooops:---")

			fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", badJSON)
			fakeRequest.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", messageMAC(signingToken, badJSON.Bytes())))
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

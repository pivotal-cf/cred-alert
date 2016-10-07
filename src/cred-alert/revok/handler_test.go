package revok_test

import (
	"bytes"
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/queue"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	Describe("ServeHTTP", func() {
		var (
			logger               *lagertest.TestLogger
			handler              http.Handler
			changeDiscoverer     *revokfakes.FakeChangeDiscoverer
			repositoryRepository *dbfakes.FakeRepositoryRepository

			fakeRequest *http.Request
			recorder    *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			recorder = httptest.NewRecorder()

			logger = lagertest.NewTestLogger("ingestor")
			changeDiscoverer = &revokfakes.FakeChangeDiscoverer{}
			repositoryRepository = &dbfakes.FakeRepositoryRepository{}
			handler = revok.NewHandler(logger, changeDiscoverer, repositoryRepository)
		})

		Context("when the payload is a valid JSON PushEventPlan", func() {
			BeforeEach(func() {
				pushEventPlan := queue.PushEventPlan{
					Owner:      "some-owner",
					Repository: "some-repo",
					From:       "from-sha",
					To:         "to-sha",
					Private:    true,
				}

				body := &bytes.Buffer{}
				err := json.NewEncoder(body).Encode(pushEventPlan)
				Expect(err).NotTo(HaveOccurred())
				fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			})

			It("looks up the repository in the database", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(repositoryRepository.FindCallCount()).To(Equal(1))
				owner, name := repositoryRepository.FindArgsForCall(0)
				Expect(owner).To(Equal("some-owner"))
				Expect(name).To(Equal("some-repo"))
			})

			Context("when the repository can be found in the database", func() {
				var (
					expectedRepository *db.Repository
				)

				BeforeEach(func() {
					expectedRepository = &db.Repository{
						Owner: "some-owner",
						Name:  "some-name",
					}

					repositoryRepository.FindReturns(*expectedRepository, nil)
				})

				It("tries to do a fetch", func() {
					handler.ServeHTTP(recorder, fakeRequest)
					Expect(changeDiscoverer.FetchCallCount()).To(Equal(1))
					_, actualRepository := changeDiscoverer.FetchArgsForCall(0)
					Expect(actualRepository).To(Equal(*expectedRepository))
				})

				Context("when the fetch succeeds", func() {
					BeforeEach(func() {
						changeDiscoverer.FetchReturns(nil)
					})

					It("responds with 202", func() {
						handler.ServeHTTP(recorder, fakeRequest)
						Expect(recorder.Code).To(Equal(http.StatusAccepted))
					})
				})

				Context("when the fetch fails", func() {
					BeforeEach(func() {
						changeDiscoverer.FetchReturns(errors.New("an-error"))
					})

					It("responds with 500", func() {
						handler.ServeHTTP(recorder, fakeRequest)
						Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when the repository can not be found in the database", func() {
				BeforeEach(func() {
					repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
				})

				It("does not try to do a fetch", func() {
					handler.ServeHTTP(recorder, fakeRequest)
					Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
				})

				It("responds with http.StatusBadRequest", func() {
					handler.ServeHTTP(recorder, fakeRequest)
					Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when the payload is not valid JSON", func() {
			BeforeEach(func() {
				bs := []byte("some bad bytes")
				body := bytes.NewReader(bs)
				fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			})

			It("does not look up the repository in the database", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(repositoryRepository.FindCallCount()).To(BeZero())
			})

			It("does not try to do a fetch", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
			})

			It("responds with http.StatusBadRequest", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the payload is a valid JSON for a PushEventPlan but is missing the repository", func() {
			BeforeEach(func() {
				bs := []byte(`{
					"owner":"some-owner"
				}`)

				body := bytes.NewReader(bs)
				fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			})

			It("does not look up the repository in the database", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(repositoryRepository.FindCallCount()).To(BeZero())
			})

			It("does not try to do a fetch", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
			})

			It("responds with http.StatusBadRequest", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the payload is a valid JSON for a PushEventPlan but is missing the owner", func() {
			BeforeEach(func() {
				bs := []byte(`{
					"repository":"some-repository"
				}`)

				body := bytes.NewReader(bs)
				fakeRequest, _ = http.NewRequest("POST", "http://example.com/webhook", body)
			})

			It("does not look up the repository in the database", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(repositoryRepository.FindCallCount()).To(BeZero())
			})

			It("does not try to do a fetch", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
			})

			It("responds with http.StatusBadRequest", func() {
				handler.ServeHTTP(recorder, fakeRequest)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
})

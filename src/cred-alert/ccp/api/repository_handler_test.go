package api_test

import (
	"cred-alert/ccp/api"
	"cred-alert/ccp/api/apifakes"
	"cred-alert/ccp/web"
	"cred-alert/revokpb"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository Handler", func() {
	var (
		fakeRevokClient *apifakes.FakeRepositoryRevokClient
		testLogger      lager.Logger

		repositoryCredentialCountResponse *revokpb.RepositoryCredentialCountResponse

		handler *api.RepositoryHandler
	)

	BeforeEach(func() {
		bs, err := web.Asset("web/templates/repository.html")
		Expect(err).NotTo(HaveOccurred())

		repositoryLayout := template.Must(template.New("repository.html").Parse(string(bs)))

		testLogger = lagertest.NewTestLogger("api")
		fakeRevokClient = &apifakes.FakeRepositoryRevokClient{}

		repositoryCredentialCountResponse = &revokpb.RepositoryCredentialCountResponse{}

		fakeRevokClient.GetRepositoryCredentialCountsReturns(repositoryCredentialCountResponse, nil)

		handler = api.NewRepositoryHandler(testLogger, repositoryLayout, fakeRevokClient)
	})

	It("renders template successfully", func() {
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "https://example.com", nil)
		Expect(err).NotTo(HaveOccurred())

		handler.ServeHTTP(w, r)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	Context("when getting credential counts returns an error", func() {
		BeforeEach(func() {
			fakeRevokClient.GetRepositoryCredentialCountsReturns(nil, errors.New("some error"))
		})

		It("renders internal server error", func() {
			w := httptest.NewRecorder()
			r, err := http.NewRequest("GET", "https://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			handler.ServeHTTP(w, r)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Context("when the template fails to render", func() {
		BeforeEach(func() {
			repositoryLayout, err := template.New("test").Parse("{{.InvalidField}} is not an expected field")
			Expect(err).NotTo(HaveOccurred())

			handler = api.NewRepositoryHandler(testLogger, repositoryLayout, fakeRevokClient)
		})

		It("returns an error", func() {
			w := httptest.NewRecorder()
			r, err := http.NewRequest("GET", "https://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			handler.ServeHTTP(w, r)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})

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

var _ = Describe("Index Handler", func() {
	var (
		fakeRevokClient *apifakes.FakeIndexRevokClient
		testLogger      lager.Logger

		credentialCountResponse *revokpb.CredentialCountResponse

		handler *api.IndexHandler
	)

	BeforeEach(func() {
		bs, err := web.Asset("web/templates/index.html")
		Expect(err).NotTo(HaveOccurred())

		indexLayout := template.Must(template.New("index.html").Parse(string(bs)))

		testLogger = lagertest.NewTestLogger("api")
		fakeRevokClient = &apifakes.FakeIndexRevokClient{}

		credentialCountResponse = &revokpb.CredentialCountResponse{}

		fakeRevokClient.GetCredentialCountsReturns(credentialCountResponse, nil)

		handler = api.NewIndexHandler(testLogger, indexLayout, fakeRevokClient)
	})

	It("renders template successfully", func() {
		w := httptest.NewRecorder()
		r := &http.Request{}
		handler.ServeHTTP(w, r)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	Context("when getting credential counts returns an error", func() {
		BeforeEach(func() {
			fakeRevokClient.GetCredentialCountsReturns(nil, errors.New("some error"))
		})

		It("renders internal server error", func() {
			w := httptest.NewRecorder()
			r := &http.Request{}
			handler.ServeHTTP(w, r)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Context("when the template fails to render", func() {
		BeforeEach(func() {
			indexLayout, err := template.New("test").Parse("{{.InvalidField}} is not an expected field")
			Expect(err).NotTo(HaveOccurred())

			handler = api.NewIndexHandler(testLogger, indexLayout, fakeRevokClient)
		})

		It("returns an error", func() {
			w := httptest.NewRecorder()
			r := &http.Request{}
			handler.ServeHTTP(w, r)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})

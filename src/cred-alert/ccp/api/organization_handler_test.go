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

var _ = Describe("Organization Handler", func() {
	var (
		fakeRevokClient *apifakes.FakeOrganizationRevokClient
		testLogger      lager.Logger

		organizationCredentialCountResponse *revokpb.OrganizationCredentialCountResponse

		handler *api.OrganizationHandler
	)

	BeforeEach(func() {
		bs, err := web.Asset("web/templates/organization.html")
		Expect(err).NotTo(HaveOccurred())

		organizationLayout := template.Must(template.New("organization.html").Parse(string(bs)))

		testLogger = lagertest.NewTestLogger("api")
		fakeRevokClient = &apifakes.FakeOrganizationRevokClient{}

		organizationCredentialCountResponse = &revokpb.OrganizationCredentialCountResponse{}

		fakeRevokClient.GetOrganizationCredentialCountsReturns(organizationCredentialCountResponse, nil)

		handler = api.NewOrganizationHandler(testLogger, organizationLayout, fakeRevokClient)
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
			fakeRevokClient.GetOrganizationCredentialCountsReturns(nil, errors.New("some error"))
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
			organizationLayout, err := template.New("test").Parse("{{.InvalidField}} is not an expected field")
			Expect(err).NotTo(HaveOccurred())

			handler = api.NewOrganizationHandler(testLogger, organizationLayout, fakeRevokClient)
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

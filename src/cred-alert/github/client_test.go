package github_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/github"
)

var _ = Describe("Client", func() {
	var (
		client github.Client
		server *ghttp.Server

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		httpClient := &http.Client{}
		client = github.NewClient(server.URL(), httpClient)

		logger = lagertest.NewTestLogger("client")
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	It("sets vnd.github.diff as the accept content-type header, and recieves a diff", func() {
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/repos/owner/repo/compare/a...b"),
				ghttp.VerifyHeader(http.Header{
					"Accept": []string{"application/vnd.github.diff"},
				}),
				ghttp.RespondWith(http.StatusOK, `THIS IS THE DIFF`),
			),
		)

		diff, err := client.CompareRefs(logger, "owner", "repo", "a", "b")
		Expect(err).NotTo(HaveOccurred())
		Expect(diff).To(Equal("THIS IS THE DIFF"))
	})

	It("returns an error if the API returns an error", func() {
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/repos/owner/repo/compare/a...b"),
				ghttp.VerifyHeader(http.Header{
					"Accept": []string{"application/vnd.github.diff"},
				}),
				ghttp.RespondWith(http.StatusInternalServerError, ""),
			),
		)

		_, err := client.CompareRefs(logger, "owner", "repo", "a", "b")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error if the API does not respond", func() {
		server.Close()
		server = nil

		_, err := client.CompareRefs(logger, "owner", "repo", "a", "b")
		Expect(err).To(HaveOccurred())
	})
})

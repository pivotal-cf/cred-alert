package githubclient_test

import (
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"cred-alert/githubclient"
)

var _ = Describe("Client", func() {
	var (
		client githubclient.Client
		server *ghttp.Server
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		httpClient := &http.Client{
			Transport: &http.Transport{},
		}

		logger = lagertest.NewTestLogger("client")
		client = githubclient.NewClient(server.URL(), httpClient)
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("CommitInfo", func() {
		var commitInfoJSON string

		BeforeEach(func() {
			commitInfoJSON = `
			{
				"commit": {
					"message": "this is a commit message"
				},
				"parents": [
					{
						"sha": "parent1",
						"url": "https://api.github.com/repos/owner/repo/commits/dea714291b6a45b03db90f96674ea15dbb0c341c",
						"html_url": "https://github.com/owner/repo/commit/dea714291b6a45b03db90f96674ea15dbb0c341c"
					},
					{
						"sha": "parent2",
						"url": "https://api.github.com/repos/owner/repo/commits/b99749ac0f3744eed8c534afa5bc46b52c280b7b",
						"html_url": "https://github.com/owner/repo/commit/b99749ac0f3744eed8c534afa5bc46b52c280b7b"
					}
				]
			}`
		})

		It("returns a list of parent shas", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/owner/repo/commits/someSha"),
					ghttp.RespondWith(http.StatusOK, commitInfoJSON),
				),
			)
			commitInfo, err := client.CommitInfo(logger, "owner", "repo", "someSha")
			Expect(err).ToNot(HaveOccurred())

			Expect(commitInfo.Parents).To(ConsistOf("parent1", "parent2"))
			Expect(commitInfo.Message).To(Equal("this is a commit message"))
		})

		Context("the api request to github fails", func() {
			It("returns and logs an error", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/commits/someSha"),
						ghttp.RespondWith(http.StatusInternalServerError, commitInfoJSON),
					),
				)

				_, err := client.CommitInfo(logger, "owner", "repo", "someSha")
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("commit-info.failed"))
			})
		})

		Context("The http client returns an error", func() {
			It("returns and logs an error", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/commits/someSha"),
						ghttp.RespondWith(http.StatusOK, commitInfoJSON),
					),
				)
				server.Close()
				server = nil
				_, err := client.CommitInfo(logger, "owner", "repo", "someSha")
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("commit-info.failed"))
			})
		})

		Context("The commit is not found", func() {
			It("logs and returns a not found error", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/commits/someSha"),
						ghttp.RespondWith(http.StatusNotFound, commitInfoJSON),
					),
				)
				_, err := client.CommitInfo(logger, "owner", "repo", "someSha")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(githubclient.ErrNotFound))
				Expect(logger).To(gbytes.Say("commit-info.failed"))
			})
		})

		Context("the response is not valid json", func() {
			It("returns and logs an error", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/commits/someSha"),
						ghttp.RespondWith(http.StatusOK, `badjson: [unbalanced: parens}`),
					),
				)

				_, err := client.CommitInfo(logger, "owner", "repo", "someSha")
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("commit-info.failed"))
			})
		})

	})

	Describe("CompareRefs", func() {
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
			Expect(ioutil.ReadAll(diff)).To(Equal([]byte("THIS IS THE DIFF")))
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

	Describe("GetArchiveLink", func() {
		var (
			header      http.Header
			zipLocation string
		)

		BeforeEach(func() {
			zipLocation = "https://github.example.com/there/is/a/file/here.zip"
			header = http.Header{}
			header.Set("Location", zipLocation)
		})

		It("returns a download link", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/owner/repo/zipball/abcdef"),
					ghttp.RespondWith(http.StatusFound, "", header),
				),
			)

			url, err := client.ArchiveLink("owner", "repo", "abcdef")
			Expect(err).ToNot(HaveOccurred())
			Expect(url.String()).To(Equal(zipLocation))
		})

		Context("When github returns an unexpected status code", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/zipball/abcdef"),
						ghttp.RespondWith(http.StatusTooManyRequests, "", header),
					),
				)
			})

			It("returns an error", func() {
				url, err := client.ArchiveLink("owner", "repo", "abcdef")
				Expect(url).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When the commit is not found", func() {
			It("returns a not found error", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/repos/owner/repo/zipball/abcdef"),
						ghttp.RespondWith(http.StatusNotFound, "", header),
					),
				)
				_, err := client.ArchiveLink("owner", "repo", "abcdef")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(githubclient.ErrNotFound))
			})
		})
	})
})

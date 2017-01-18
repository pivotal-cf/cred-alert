package revok_test

import (
	"context"
	"errors"
	"io"
	"net"

	"code.cloudfoundry.org/lager/lagertest"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/search/searchfakes"
)

var _ = Describe("Server", func() {
	var (
		repoDB   *dbfakes.FakeRepositoryRepository
		searcher *searchfakes.FakeSearcher
		server   revok.Server

		ctx     context.Context
		request *revokpb.CredentialCountRequest

		response *revokpb.CredentialCountResponse
		err      error
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("revok-server")
		repoDB = &dbfakes.FakeRepositoryRepository{}
		searcher = &searchfakes.FakeSearcher{}
		ctx = context.Background()

		server = revok.NewServer(logger, repoDB, searcher)
	})

	Describe("GetCredentialCounts", func() {
		BeforeEach(func() {
			repoDB.AllReturns([]db.Repository{
				{
					Owner: "some-owner",
					CredentialCounts: db.PropertyMap{
						"o1r1b1": float64(1),
						"o1r1b2": float64(2),
					},
				},
				{
					Owner: "some-owner",
					CredentialCounts: db.PropertyMap{
						"o1r2b1": float64(3),
						"o1r2b2": float64(4),
					},
				},
				{
					Owner: "some-other-owner",
					CredentialCounts: db.PropertyMap{
						"o2b1": float64(5),
						"o2b2": float64(6),
					},
				},
			}, nil)

			request = &revokpb.CredentialCountRequest{}
		})

		JustBeforeEach(func() {
			response, err = server.GetCredentialCounts(ctx, request)
		})

		It("gets repositories from the database", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repoDB.AllCallCount()).To(Equal(1))
		})

		It("returns counts for all repositories in owner-alphabetical order", func() {
			occ1 := &revokpb.OrganizationCredentialCount{
				Owner: "some-other-owner",
				Count: 11,
			}

			occ2 := &revokpb.OrganizationCredentialCount{
				Owner: "some-owner",
				Count: 10,
			}

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.OrganizationCredentialCount{occ1, occ2}))
		})
	})

	Describe("Search", func() {
		var (
			grpcServer  *grpc.Server
			listener    net.Listener
			revokClient revokpb.RevokClient
			connection  *grpc.ClientConn
		)

		BeforeEach(func() {
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())

			grpcServer = grpc.NewServer()
			revokpb.RegisterRevokServer(grpcServer, server)

			go grpcServer.Serve(listener)

			connection, err = grpc.Dial(listener.Addr().String(), grpc.WithInsecure())
			Expect(err).NotTo(HaveOccurred())

			revokClient = revokpb.NewRevokClient(connection)
		})

		AfterEach(func() {
			connection.Close()
			grpcServer.Stop()
		})

		It("searches using a matcher", func() {
			resultsChan := make(chan search.Result, 10)

			resultsChan <- search.Result{
				Owner:      "owner-name",
				Repository: "repo-name",
				Revision:   "abc123",
				Path:       "thing.txt",
				LineNumber: 14,
				Location:   3,
				Length:     23,
				Content:    []byte("awesome content"),
			}

			resultsChan <- search.Result{
				Owner:      "owner-name",
				Repository: "other-repo-name",
				Revision:   "def456",
				Path:       "another-thing.txt",
				LineNumber: 91999,
				Location:   93,
				Length:     12,
				Content:    []byte("totally a credential"),
			}

			close(resultsChan)

			searchResults := &searchfakes.FakeResults{}
			searchResults.CReturns(resultsChan)

			searcher.SearchCurrentReturns(searchResults)

			stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(searcher.SearchCurrentCallCount).Should(Equal(1))

			results := []revokpb.SearchResult{}

			for {
				result, err := stream.Recv()
				if err == io.EOF {
					break
				}

				Expect(err).NotTo(HaveOccurred())

				results = append(results, *result)
			}

			Expect(results).To(ConsistOf(revokpb.SearchResult{
				Location: &revokpb.SourceLocation{
					Repository: &revokpb.Repository{
						Owner: "owner-name",
						Name:  "other-repo-name",
					},
					Revision:   "def456",
					Path:       "another-thing.txt",
					LineNumber: 91999,
					Location:   93,
					Length:     12,
				},
				Content: "totally a credential",
			},
				revokpb.SearchResult{
					Location: &revokpb.SourceLocation{
						Repository: &revokpb.Repository{
							Owner: "owner-name",
							Name:  "repo-name",
						},
						Revision:   "abc123",
						Path:       "thing.txt",
						LineNumber: 14,
						Location:   3,
						Length:     23,
					},
					Content: "awesome content",
				},
			))
		})

		Context("when there is an error getting all of the results", func() {
			It("returns an error to the client", func() {
				resultsChan := make(chan search.Result, 10)
				close(resultsChan)

				searchResults := &searchfakes.FakeResults{}
				searchResults.CReturns(resultsChan)
				searchResults.ErrReturns(errors.New("disaster"))

				searcher.SearchCurrentReturns(searchResults)

				stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(searcher.SearchCurrentCallCount).Should(Equal(1))

				result, err := stream.Recv()
				Expect(err).To(MatchError(ContainSubstring("disaster")))
				Expect(result).To(BeNil())
			})
		})
	})
})

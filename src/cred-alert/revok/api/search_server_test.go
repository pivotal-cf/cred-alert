package api_test

import (
	"context"
	"errors"
	"io"
	"net"

	"code.cloudfoundry.org/lager/lagertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/revok/api"
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/search/searchfakes"
	"red/redpb"
)

var _ = Describe("SearchServer", func() {
	var (
		blobSearcher *searchfakes.FakeBlobSearcher
		searcher     *searchfakes.FakeSearcher
		s            *api.SearchServer
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("revok-server")

		blobSearcher = &searchfakes.FakeBlobSearcher{}
		searcher = &searchfakes.FakeSearcher{}

		s = api.NewSearchServer(logger, searcher, blobSearcher)
	})

	Describe("BoshBlobs", func() {
		var (
			response *revokpb.BoshBlobsResponse
			err      error
		)

		BeforeEach(func() {
			blobSearcher.ListBlobsReturns([]search.BlobResult{
				{
					Path: "golang/golang.tgz",
					SHA:  "123abc",
				},
			}, nil)

			request := &revokpb.BoshBlobsRequest{
				Repository: &redpb.Repository{
					Owner: "owner-name",
					Name:  "repo-name",
				},
			}
			response, err = s.BoshBlobs(context.Background(), request)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the blobs", func() {
			blob := &revokpb.BoshBlob{
				Path: "golang/golang.tgz",
				Sha:  "123abc",
			}

			Expect(response).NotTo(BeNil())
			Expect(response.Blobs).NotTo(BeNil())
			Expect(response.Blobs).To(Equal([]*revokpb.BoshBlob{blob}))
		})

		Context("when the database returns an error", func() {
			BeforeEach(func() {
				blobSearcher.ListBlobsReturns(nil, errors.New("disaster"))
			})

			It("errors", func() {
				request := &revokpb.BoshBlobsRequest{}
				_, err = s.BoshBlobs(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Search", func() {
		var (
			grpcServer  *grpc.Server
			listener    net.Listener
			revokClient revokpb.RevokSearchClient
			connection  *grpc.ClientConn
		)

		BeforeEach(func() {
			var err error
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())

			grpcServer = grpc.NewServer()
			revokpb.RegisterRevokSearchServer(grpcServer, s)

			go grpcServer.Serve(listener)

			connection, err = grpc.Dial(
				listener.Addr().String(),
				grpc.WithInsecure(),
				grpc.WithBlock(),
			)
			Expect(err).NotTo(HaveOccurred())

			revokClient = revokpb.NewRevokSearchClient(connection)
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

			stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{
				Regex: "hello, (.*)",
			})
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
					Repository: &redpb.Repository{
						Owner: "owner-name",
						Name:  "other-repo-name",
					},
					Revision:   "def456",
					Path:       "another-thing.txt",
					LineNumber: 91999,
					Location:   93,
					Length:     12,
				},
				Content: []byte("totally a credential"),
			},
				revokpb.SearchResult{
					Location: &revokpb.SourceLocation{
						Repository: &redpb.Repository{
							Owner: "owner-name",
							Name:  "repo-name",
						},
						Revision:   "abc123",
						Path:       "thing.txt",
						LineNumber: 14,
						Location:   3,
						Length:     23,
					},
					Content: []byte("awesome content"),
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

				stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{
					Regex: "hello, (.*)",
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(searcher.SearchCurrentCallCount).Should(Equal(1))

				result, err := stream.Recv()
				Expect(err).To(MatchError(ContainSubstring("disaster")))
				Expect(result).To(BeNil())
			})
		})

		Context("with an empty regular expression", func() {
			It("returns an error", func() {
				resultsChan := make(chan search.Result, 10)
				close(resultsChan)

				searchResults := &searchfakes.FakeResults{}
				searchResults.CReturns(resultsChan)

				searcher.SearchCurrentReturns(searchResults)

				stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{
					Regex: "",
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = stream.Recv()
				Expect(err).To(MatchError(ContainSubstring("query regular expression may not be empty")))
				Expect(grpc.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})

		Context("with an invalid regular expression", func() {
			It("returns an error", func() {
				resultsChan := make(chan search.Result, 10)
				close(resultsChan)

				searchResults := &searchfakes.FakeResults{}
				searchResults.CReturns(resultsChan)

				searcher.SearchCurrentReturns(searchResults)

				stream, err := revokClient.Search(context.Background(), &revokpb.SearchQuery{
					Regex: "((",
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = stream.Recv()
				Expect(err).To(MatchError(ContainSubstring("query regular expression is invalid: '(('")))
				Expect(grpc.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})
})

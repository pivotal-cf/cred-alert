package revok_test

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

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/search/searchfakes"
	"red/redpb"
)

var _ = Describe("Server", func() {
	var (
		repositoryRepository *dbfakes.FakeRepositoryRepository
		branchRepository     *dbfakes.FakeBranchRepository

		blobSearcher *searchfakes.FakeBlobSearcher
		searcher     *searchfakes.FakeSearcher
		server       revok.Server
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("revok-server")

		branchRepository = &dbfakes.FakeBranchRepository{}
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		blobSearcher = &searchfakes.FakeBlobSearcher{}
		searcher = &searchfakes.FakeSearcher{}

		server = revok.NewServer(logger, searcher, blobSearcher, repositoryRepository, branchRepository)
	})

	Describe("GetCredentialCounts", func() {
		var (
			response *revokpb.CredentialCountResponse
			err      error
		)

		BeforeEach(func() {
			branchRepository.GetCredentialCountByOwnerReturns([]db.OwnerCredentialCount{
				{
					Owner:           "some-other-owner",
					CredentialCount: 11,
				},
				{
					Owner:           "some-owner",
					CredentialCount: 10,
				},
			}, nil)

			request := &revokpb.CredentialCountRequest{}
			response, err = server.GetCredentialCounts(context.Background(), request)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
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

		Context("when the database returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountByOwnerReturns(nil, errors.New("disaster"))
			})

			It("errors", func() {
				request := &revokpb.CredentialCountRequest{}
				_, err = server.GetCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GetOrganizationCredentialCounts", func() {
		var (
			response *revokpb.OrganizationCredentialCountResponse
			err      error
		)

		BeforeEach(func() {
			branchRepository.GetCredentialCountForOwnerReturns([]db.RepositoryCredentialCount{
				{
					Owner:           "some-owner",
					Name:            "repo-name-1",
					CredentialCount: 14,
					Private:         true,
				},
				{
					Owner:           "some-owner",
					Name:            "repo-name-2",
					CredentialCount: 2,
					Private:         false,
				},
			}, nil)

			request := &revokpb.OrganizationCredentialCountRequest{}
			response, err = server.GetOrganizationCredentialCounts(context.Background(), request)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns counts and privacy value for all repositories in owner-alphabetical order", func() {
			occ1 := &revokpb.RepositoryCredentialCount{
				Owner:   "some-owner",
				Name:    "repo-name-1",
				Private: true,
				Count:   14,
			}

			occ2 := &revokpb.RepositoryCredentialCount{
				Owner:   "some-owner",
				Name:    "repo-name-2",
				Private: false,
				Count:   2,
			}

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.RepositoryCredentialCount{occ1, occ2}))
		})

		Context("when the database returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountForOwnerReturns(nil, errors.New("disaster"))
			})

			It("errors", func() {
				request := &revokpb.OrganizationCredentialCountRequest{}
				_, err = server.GetOrganizationCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GetRepositoryCredentialCounts", func() {
		BeforeEach(func() {
			branchRepository.GetCredentialCountForRepoReturns([]db.BranchCredentialCount{
				{
					Owner:           "some-owner",
					Name:            "repo-name",
					Branch:          "branch-1",
					CredentialCount: 14,
				},
				{
					Owner:           "some-owner",
					Name:            "repo-name",
					Branch:          "branch-2",
					CredentialCount: 2,
				},
			}, nil)

			repositoryRepository.FindReturns(
				db.Repository{
					Name:    "some-repo",
					Private: true,
				},
				true,
				nil,
			)

		})

		It("returns counts for all branches in alphabetical order", func() {
			occ1 := &revokpb.BranchCredentialCount{
				Name:  "branch-1",
				Count: 14,
			}

			occ2 := &revokpb.BranchCredentialCount{
				Name:  "branch-2",
				Count: 2,
			}

			request := &revokpb.RepositoryCredentialCountRequest{}
			response, err := server.GetRepositoryCredentialCounts(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.BranchCredentialCount{occ1, occ2}))
		})

		Context("when looking up credential counts returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountForRepoReturns(
					nil,
					errors.New("credential count disaster"),
				)
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := server.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when looking up repository returns an error", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, false, errors.New("repository disaster"))
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := server.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when repository is not found", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, false, nil)
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := server.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})
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
			response, err = server.BoshBlobs(context.Background(), request)
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
				_, err = server.BoshBlobs(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
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
			var err error
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

				_, err = stream.Recv()
				Expect(err).To(MatchError(ContainSubstring("query regular expression is invalid: '(('")))
				Expect(grpc.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})
})

package server

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"red/redpb"

	"cred-alert/db"
	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/sniff/matchers"
)

//go:generate bash $GOPATH/scripts/generate_protos.sh

type Server struct {
	logger       lager.Logger
	searcher     search.Searcher
	blobSearcher search.BlobSearcher

	repositoryRepository db.RepositoryRepository
	branchRepository     db.BranchRepository
}

func NewServer(
	logger lager.Logger,
	searcher search.Searcher,
	blobSearcher search.BlobSearcher,
	repositoryRepository db.RepositoryRepository,
	branchRepository db.BranchRepository,
) *Server {
	return &Server{
		logger:               logger,
		searcher:             searcher,
		blobSearcher:         blobSearcher,
		repositoryRepository: repositoryRepository,
		branchRepository:     branchRepository,
	}
}

func (s *Server) GetCredentialCounts(
	ctx context.Context,
	in *revokpb.CredentialCountRequest,
) (*revokpb.CredentialCountResponse, error) {
	logger := s.logger.Session("get-all-credential-counts")

	credentialsByOwner, err := s.branchRepository.GetCredentialCountByOwner()
	if err != nil {
		logger.Error("failed-to-get-all-credential-counts", err)
		return nil, err
	}

	response := &revokpb.CredentialCountResponse{}

	for _, report := range credentialsByOwner {
		response.CredentialCounts = append(response.CredentialCounts, &revokpb.OrganizationCredentialCount{
			Owner: report.Owner,
			Count: int64(report.CredentialCount),
		})
	}

	return response, nil
}

func (s *Server) GetOrganizationCredentialCounts(
	ctx context.Context,
	in *revokpb.OrganizationCredentialCountRequest,
) (*revokpb.OrganizationCredentialCountResponse, error) {
	logger := s.logger.Session("get-owner-credential-counts", lager.Data{
		"owner": in.GetOwner(),
	})

	credentialsForOwner, err := s.branchRepository.GetCredentialCountForOwner(in.GetOwner())
	if err != nil {
		logger.Error("failed-to-get-owner-credential-count", err)
		return nil, err
	}

	rccs := []*revokpb.RepositoryCredentialCount{}

	for _, report := range credentialsForOwner {
		rccs = append(rccs, &revokpb.RepositoryCredentialCount{
			Owner:   report.Owner,
			Name:    report.Name,
			Private: report.Private,
			Count:   int64(report.CredentialCount),
		})
	}

	response := &revokpb.OrganizationCredentialCountResponse{
		CredentialCounts: rccs,
	}

	return response, nil
}

func (s *Server) GetRepositoryCredentialCounts(
	ctx context.Context,
	in *revokpb.RepositoryCredentialCountRequest,
) (*revokpb.RepositoryCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repository, found, err := s.repositoryRepository.Find(in.GetOwner(), in.GetName())
	if err != nil {
		logger.Error("failed-to-get-repository", err)
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("Repository not found: %s", in.GetName())
	}

	credentialsForOwner, err := s.branchRepository.GetCredentialCountForRepo(in.GetOwner(), in.GetName())
	if err != nil {
		logger.Error("failed-to-get-repository-credential-count", err)
		return nil, err
	}

	bccs := []*revokpb.BranchCredentialCount{}

	for _, report := range credentialsForOwner {
		bccs = append(bccs, &revokpb.BranchCredentialCount{
			Name:  report.Branch,
			Count: int64(report.CredentialCount),
		})
	}

	response := &revokpb.RepositoryCredentialCountResponse{
		CredentialCounts: bccs,
		Private:          repository.Private,
	}

	return response, nil
}

func (s *Server) BoshBlobs(ctx context.Context, request *revokpb.BoshBlobsRequest) (*revokpb.BoshBlobsResponse, error) {
	logger := s.logger.Session("bosh-blobs-endpoint")

	repository := request.GetRepository()
	blobs, err := s.blobSearcher.ListBlobs(logger, repository.GetOwner(), repository.GetName())
	if err != nil {
		return nil, err
	}

	response := revokpb.BoshBlobsResponse{}

	for _, blob := range blobs {
		response.Blobs = append(response.Blobs, &revokpb.BoshBlob{
			Path: blob.Path,
			Sha:  blob.SHA,
		})
	}

	return &response, nil
}

func (s *Server) Search(query *revokpb.SearchQuery, stream revokpb.Revok_SearchServer) error {
	logger := s.logger.Session("search-endpoint")
	logger.Info("hit")

	regex := query.GetRegex()
	if regex == "" {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression may not be empty")
	}

	matcher, err := matchers.TryFormat(regex)
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression is invalid: '%s'", regex)
	}

	results := s.searcher.SearchCurrent(stream.Context(), logger, matcher)

	for result := range results.C() {
		searchResult := &revokpb.SearchResult{
			Location: &revokpb.SourceLocation{
				Repository: &redpb.Repository{
					Owner: result.Owner,
					Name:  result.Repository,
				},
				Revision:   result.Revision,
				Path:       result.Path,
				LineNumber: uint32(result.LineNumber),
				Location:   uint32(result.Location),
				Length:     uint32(result.Length),
			},
			Content: result.Content,
		}

		if err := stream.Send(searchResult); err != nil {
			return err
		}
	}

	if err := results.Err(); err != nil {
		return err
	}

	return nil
}

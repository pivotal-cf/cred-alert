package revok

import (
	"sort"

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

//go:generate go-bindata -o web/bindata.go -ignore bindata -pkg web web/templates/...

//go:generate counterfeiter . Server

type Server interface {
	revokpb.RevokServer
}

type server struct {
	logger   lager.Logger
	searcher search.Searcher

	repositoryRepository db.RepositoryRepository
	branchRepository     db.BranchRepository
}

func NewServer(
	logger lager.Logger,
	searcher search.Searcher,
	repositoryRepository db.RepositoryRepository,
	branchRepository db.BranchRepository,
) Server {
	return &server{
		logger:               logger,
		searcher:             searcher,
		repositoryRepository: repositoryRepository,
		branchRepository:     branchRepository,
	}
}

func (s *server) GetCredentialCounts(
	ctx context.Context,
	in *revokpb.CredentialCountRequest,
) (*revokpb.CredentialCountResponse, error) {
	logger := s.logger.Session("get-organization-credential-counts")

	credentialsByOwner, err := s.branchRepository.GetCredentialCountByOwner()
	if err != nil {
		logger.Error("failed-to-get-owner-credential-counts", err)
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

func (s *server) GetOrganizationCredentialCounts(
	ctx context.Context,
	in *revokpb.OrganizationCredentialCountRequest,
) (*revokpb.OrganizationCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repositories, err := s.repositoryRepository.AllForOrganization(in.Owner)
	if err != nil {
		logger.Error("failed-getting-repositories-from-db", err)
		return nil, err
	}

	rccs := []*revokpb.RepositoryCredentialCount{}
	for _, repository := range repositories {
		var count int64

		branches, err := s.branchRepository.GetBranches(repository)
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			count += int64(branch.CredentialCount)
		}

		rccs = append(rccs, &revokpb.RepositoryCredentialCount{
			Owner: repository.Owner,
			Name:  repository.Name,
			Count: count,
		})
	}

	sort.Sort(revokpb.RCCByName(rccs))

	response := &revokpb.OrganizationCredentialCountResponse{
		CredentialCounts: rccs,
	}

	return response, nil
}

func (s *server) GetRepositoryCredentialCounts(
	ctx context.Context,
	in *revokpb.RepositoryCredentialCountRequest,
) (*revokpb.RepositoryCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repository, err := s.repositoryRepository.MustFind(in.Owner, in.Name)
	if err != nil {
		logger.Error("failed-getting-repository-from-db", err)
		return nil, err
	}

	bccs := []*revokpb.BranchCredentialCount{}

	branches, err := s.branchRepository.GetBranches(repository)
	if err != nil {
		return nil, err
	}

	for _, branch := range branches {
		bccs = append(bccs, &revokpb.BranchCredentialCount{
			Name:  branch.Name,
			Count: int64(branch.CredentialCount),
		})
	}

	sort.Sort(revokpb.BCCByName(bccs))

	response := &revokpb.RepositoryCredentialCountResponse{
		CredentialCounts: bccs,
	}

	return response, nil
}

func (s *server) Search(query *revokpb.SearchQuery, stream revokpb.Revok_SearchServer) error {
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

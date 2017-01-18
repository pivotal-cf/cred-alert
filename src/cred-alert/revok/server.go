package revok

import (
	"sort"

	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

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
	logger         lager.Logger
	repoRepository db.RepositoryRepository
	searcher       search.Searcher
}

func NewServer(logger lager.Logger, repoRepository db.RepositoryRepository, searcher search.Searcher) Server {
	return &server{
		logger:         logger,
		repoRepository: repoRepository,
		searcher:       searcher,
	}
}

func (s *server) GetCredentialCounts(
	ctx context.Context,
	in *revokpb.CredentialCountRequest,
) (*revokpb.CredentialCountResponse, error) {
	logger := s.logger.Session("get-organization-credential-counts")

	repositories, err := s.repoRepository.All()
	if err != nil {
		logger.Error("failed-getting-repositories-from-db", err)
		return nil, err
	}

	orgCounts := map[string]float64{}
	for i := range repositories {
		for _, branchCountInt := range repositories[i].CredentialCounts {
			if branchCount, ok := branchCountInt.(float64); ok {
				orgCounts[repositories[i].Owner] += branchCount
			}
		}
	}

	orgNames := []string{}
	for name := range orgCounts {
		orgNames = append(orgNames, name)
	}
	sort.Strings(orgNames)

	response := &revokpb.CredentialCountResponse{}
	for _, orgName := range orgNames {
		occ := &revokpb.OrganizationCredentialCount{
			Owner: orgName,
			Count: int64(orgCounts[orgName]),
		}
		response.CredentialCounts = append(response.CredentialCounts, occ)
	}

	return response, nil
}

func (s *server) GetOrganizationCredentialCounts(
	ctx context.Context,
	in *revokpb.OrganizationCredentialCountRequest,
) (*revokpb.OrganizationCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repositories, err := s.repoRepository.AllForOrganization(in.Owner)
	if err != nil {
		logger.Error("failed-getting-repositories-from-db", err)
		return nil, err
	}

	rccs := []*revokpb.RepositoryCredentialCount{}
	for i := range repositories {
		var count int64
		for _, branchCountInt := range repositories[i].CredentialCounts {
			if branchCount, ok := branchCountInt.(float64); ok {
				count += int64(branchCount)
			}
		}

		rccs = append(rccs, &revokpb.RepositoryCredentialCount{
			Owner: repositories[i].Owner,
			Name:  repositories[i].Name,
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

	repository, err := s.repoRepository.Find(in.Owner, in.Name)
	if err != nil {
		logger.Error("failed-getting-repository-from-db", err)
		return nil, err
	}

	bccs := []*revokpb.BranchCredentialCount{}
	for branch, countInt := range repository.CredentialCounts {
		if count, ok := countInt.(float64); ok {
			bccs = append(bccs, &revokpb.BranchCredentialCount{
				Name:  branch,
				Count: int64(count),
			})
		}
	}

	sort.Sort(revokpb.BCCByName(bccs))

	response := &revokpb.RepositoryCredentialCountResponse{
		CredentialCounts: bccs,
	}

	return response, nil
}

func (s *server) Search(query *revokpb.SearchQuery, stream revokpb.Revok_SearchServer) error {
	regex := query.GetRegex()
	if regex == "" {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression may not be empty")
	}

	matcher, err := matchers.TryFormat(regex)
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression is invalid: '%s'", regex)
	}

	results := s.searcher.SearchCurrent(matcher)

	for result := range results.C() {
		searchResult := &revokpb.SearchResult{
			Location: &revokpb.SourceLocation{
				Repository: &revokpb.Repository{
					Owner: result.Owner,
					Name:  result.Repository,
				},
				Revision:   result.Revision,
				Path:       result.Path,
				LineNumber: uint32(result.LineNumber),
				Location:   uint32(result.Location),
				Length:     uint32(result.Length),
			},
			Content: string(result.Content),
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

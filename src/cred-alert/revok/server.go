package revok

import (
	"cred-alert/db"
	"cred-alert/revokpb"
	"fmt"
	"sort"
	"time"

	"code.cloudfoundry.org/lager"

	"golang.org/x/net/context"
)

//go:generate bash $GOPATH/scripts/generate_protos.sh

//go:generate go-bindata -o web/bindata.go -ignore bindata -pkg web web/templates/...

//go:generate counterfeiter . Server

type Server interface {
	revokpb.RevokServer
}

type server struct {
	logger lager.Logger
	db     db.RepositoryRepository
}

func NewServer(logger lager.Logger, db db.RepositoryRepository) Server {
	return &server{
		logger: logger,
		db:     db,
	}
}

func (s *server) GetCredentialCounts(
	ctx context.Context,
	in *revokpb.CredentialCountRequest,
) (*revokpb.CredentialCountResponse, error) {
	logger := s.logger.Session("get-organization-credential-counts")

	repositories, err := s.db.All()
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

	repositories, err := s.db.AllForOrganization(in.Owner)
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

	repository, err := s.db.Find(in.Owner, in.Name)
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
	for i := 0; i < 42; i++ {
		searchResult := &revokpb.SearchResult{
			Location: &revokpb.SourceLocation{
				Repository: &revokpb.Repository{
					Owner: "pivotal-cf",
					Name:  fmt.Sprintf("repository-%d", i),
				},
				Revision:   "adc83b19e793491b1c6ea0fd8b46cd9f32e592fc",
				Path:       "my/special/file.go",
				LineNumber: uint32(i),
				Location:   uint32(i * 2),
				Length:     uint32(i * 3),
			},
			Content: fmt.Sprintf("My Special Content %d", i),
		}

		if err := stream.Send(searchResult); err != nil {
			return err
		}

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

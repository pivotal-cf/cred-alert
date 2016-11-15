package revok

import (
	"cred-alert/db"
	"cred-alert/revokpb"
	"sort"

	"code.cloudfoundry.org/lager"

	"golang.org/x/net/context"
)

//go:generate bash $GOPATH/scripts/generate_protos.sh

//go:generate go-bindata -o web/bindata.go -ignore bindata -pkg web web/templates/...

//go:generate counterfeiter . RevokServer

type RevokServer interface {
	GetCredentialCounts(context.Context, *revokpb.CredentialCountRequest) (*revokpb.CredentialCountResponse, error)
	GetOrganizationCredentialCounts(context.Context, *revokpb.OrganizationCredentialCountRequest) (*revokpb.OrganizationCredentialCountResponse, error)
}

type revokServer struct {
	logger lager.Logger
	db     db.RepositoryRepository
}

func NewRevokServer(logger lager.Logger, db db.RepositoryRepository) RevokServer {
	return &revokServer{
		logger: logger,
		db:     db,
	}
}

func (s *revokServer) GetCredentialCounts(
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
	for name, _ := range orgCounts {
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

func (s *revokServer) GetOrganizationCredentialCounts(
	ctx context.Context,
	in *revokpb.OrganizationCredentialCountRequest,
) (*revokpb.OrganizationCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repositories, err := s.db.AllForOrganization(in.Owner)
	if err != nil {
		logger.Error("failed-getting-repositories-from-db", err)
		return nil, err
	}

	repoCounts := map[string]float64{}
	for i := range repositories {
		for _, branchCountInt := range repositories[i].CredentialCounts {
			if branchCount, ok := branchCountInt.(float64); ok {
				repoCounts[repositories[i].Name] += branchCount
			}
		}
	}

	response := &revokpb.OrganizationCredentialCountResponse{}
	for repo, count := range repoCounts {
		occ := &revokpb.RepositoryCredentialCount{
			Name:  repo,
			Count: int64(count),
		}

		response.CredentialCounts = append(response.CredentialCounts, occ)
	}

	return response, nil
}

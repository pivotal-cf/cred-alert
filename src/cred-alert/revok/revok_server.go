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
	GetRepositoryCredentialCounts(ctx context.Context, in *revokpb.RepositoryCredentialCountRequest) (*revokpb.RepositoryCredentialCountResponse, error)
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

	repoCounts := map[*db.Repository]float64{}
	for i := range repositories {
		for _, branchCountInt := range repositories[i].CredentialCounts {
			if branchCount, ok := branchCountInt.(float64); ok {
				repoCounts[&repositories[i]] += branchCount
			}
		}
	}

	response := &revokpb.OrganizationCredentialCountResponse{}
	for repository, count := range repoCounts {
		occ := &revokpb.RepositoryCredentialCount{
			Owner: repository.Owner,
			Name:  repository.Name,
			Count: int64(count),
		}

		response.CredentialCounts = append(response.CredentialCounts, occ)
	}

	return response, nil
}

func (s *revokServer) GetRepositoryCredentialCounts(
	ctx context.Context,
	in *revokpb.RepositoryCredentialCountRequest,
) (*revokpb.RepositoryCredentialCountResponse, error) {
	logger := s.logger.Session("get-repository-credential-counts")

	repository, err := s.db.Find(in.Owner, in.Name)
	if err != nil {
		logger.Error("failed-getting-repository-from-db", err)
		return nil, err
	}

	branchCounts := map[string]float64{}
	for branch, countInt := range repository.CredentialCounts {
		if branchCount, ok := countInt.(float64); ok {
			branchCounts[branch] += branchCount
		}
	}

	response := &revokpb.RepositoryCredentialCountResponse{}
	for branch, count := range branchCounts {
		occ := &revokpb.BranchCredentialCount{
			Name:  branch,
			Count: int64(count),
		}

		response.CredentialCounts = append(response.CredentialCounts, occ)
	}

	return response, nil
}

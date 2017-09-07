package api

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"

	"cred-alert/db"
	"cred-alert/revokpb"
)

type Server struct {
	logger               lager.Logger
	repositoryRepository db.RepositoryRepository
	branchRepository     db.BranchRepository
}

func NewServer(
	logger lager.Logger,
	repositoryRepository db.RepositoryRepository,
	branchRepository db.BranchRepository,
) *Server {
	return &Server{
		logger:               logger,
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

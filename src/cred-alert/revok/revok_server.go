package revok

import (
	"cred-alert/db"
	"cred-alert/revokpb"

	"code.cloudfoundry.org/lager"

	"golang.org/x/net/context"
)

//go:generate bash $GOPATH/scripts/generate_protos.sh

//go:generate counterfeiter . RevokServer

type RevokServer interface {
	GetCredentialCounts(context.Context, *revokpb.CredentialCountRequest) (*revokpb.CredentialCountResponse, error)
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

func (s *revokServer) GetCredentialCounts(ctx context.Context, in *revokpb.CredentialCountRequest) (*revokpb.CredentialCountResponse, error) {
	logger := s.logger.Session("get-credential-counts")

	repositories, err := s.db.All()
	if err != nil {
		logger.Error("failed-getting-repositories-from-db", err)
		return nil, err
	}

	response := &revokpb.CredentialCountResponse{}
	for i := range repositories {
		response.CredentialCounts = append(response.CredentialCounts, &revokpb.CredentialCount{
			Owner:      repositories[i].Owner,
			Repository: repositories[i].Name,
			Count:      repositories[i].CredentialCount,
		})

	}

	return response, nil
}

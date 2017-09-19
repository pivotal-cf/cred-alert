package api

import (
	"bytes"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	context "golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . RepositoryRevokClient

type RepositoryRevokClient interface {
	GetRepositoryCredentialCounts(ctx context.Context, in *revokpb.RepositoryCredentialCountRequest, opts ...grpc.CallOption) (*revokpb.RepositoryCredentialCountResponse, error)
}

type RepositoryHandler struct {
	logger   lager.Logger
	template *template.Template
	client   RepositoryRevokClient
}

func NewRepositoryHandler(
	logger lager.Logger,
	template *template.Template,
	client RepositoryRevokClient,
) *RepositoryHandler {
	return &RepositoryHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Branch struct {
	Name            string
	CredentialCount int64
}

type BranchesRepository struct {
	Branches []*Branch
	Private  bool
}

func (h *RepositoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := &revokpb.RepositoryCredentialCountRequest{
		Owner: rata.Param(r, "organization"),
		Name:  rata.Param(r, "repository"),
	}

	response, err := h.client.GetRepositoryCredentialCounts(context.Background(), request)
	if err != nil {
		h.logger.Error("failed-to-get-organization-credential-counts", err)
		if grpc.Code(err) == codes.NotFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	branches := []*Branch{}
	for _, r := range response.CredentialCounts {
		branches = append(branches, &Branch{
			Name:            r.Name,
			CredentialCount: r.Count,
		})
	}

	repo := BranchesRepository{
		Branches: branches,
		Private:  response.Private,
	}

	buf := bytes.NewBuffer([]byte{})
	err = h.template.Execute(buf, repo)
	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

package api

import (
	"bytes"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	context "golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . OrganizationRevokClient

type OrganizationRevokClient interface {
	GetOrganizationCredentialCounts(ctx context.Context, in *revokpb.OrganizationCredentialCountRequest, opts ...grpc.CallOption) (*revokpb.OrganizationCredentialCountResponse, error)
}

type OrganizationHandler struct {
	logger   lager.Logger
	template *template.Template
	client   OrganizationRevokClient
}

func NewOrganizationHandler(
	logger lager.Logger,
	template *template.Template,
	client OrganizationRevokClient,
) *OrganizationHandler {
	return &OrganizationHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Repository struct {
	Owner           string
	Name            string
	Private         bool
	CredentialCount int64
}

func (h *OrganizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := &revokpb.OrganizationCredentialCountRequest{
		Owner: rata.Param(r, "organization"),
	}

	response, err := h.client.GetOrganizationCredentialCounts(context.Background(), request)
	if err != nil {
		h.logger.Error("failed-to-get-organization-credential-counts", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	repos := []*Repository{}
	for _, r := range response.CredentialCounts {
		repos = append(repos, &Repository{
			Owner:           r.Owner,
			Name:            r.Name,
			CredentialCount: r.Count,
			Private:         r.Private,
		})
	}

	buf := bytes.NewBuffer([]byte{})
	err = h.template.Execute(buf, repos)
	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

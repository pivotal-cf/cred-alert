package api

import (
	"bytes"
	"context"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/lager"
)

type organizationHandler struct {
	logger   lager.Logger
	template *template.Template
	client   revokpb.RevokClient
}

func NewOrganizationHandler(
	logger lager.Logger,
	template *template.Template,
	client revokpb.RevokClient,
) http.Handler {
	return &organizationHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Repository struct {
	Name            string
	CredentialCount int64
}

func (h *organizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			Name:            r.Name,
			CredentialCount: r.Count,
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

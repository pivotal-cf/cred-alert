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

type repositoryHandler struct {
	logger   lager.Logger
	template *template.Template
	client   revokpb.RevokClient
}

func NewRepositoryHandler(
	logger lager.Logger,
	template *template.Template,
	client revokpb.RevokClient,
) http.Handler {
	return &repositoryHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Branch struct {
	Name            string
	CredentialCount int64
}

func (h *repositoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := &revokpb.RepositoryCredentialCountRequest{
		Owner: rata.Param(r, "organization"),
		Name:  rata.Param(r, "repository"),
	}

	response, err := h.client.GetRepositoryCredentialCounts(context.Background(), request)
	if err != nil {
		h.logger.Error("failed-to-get-organization-credential-counts", err)
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

	buf := bytes.NewBuffer([]byte{})
	err = h.template.Execute(buf, branches)
	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

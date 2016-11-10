package api

import (
	"bytes"
	"context"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type indexHandler struct {
	logger   lager.Logger
	template *template.Template
	client   revokpb.RevokClient
}

func NewIndexHandler(
	logger lager.Logger,
	template *template.Template,
	client revokpb.RevokClient,
) http.Handler {
	return &indexHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Repository struct {
	Name            string
	CredentialCount uint32
}

type Organization struct {
	Name            string
	Repositories    []Repository
	CredentialCount uint32
}

type TemplateData struct {
	Organizations []*Organization
}

func (h indexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := &revokpb.CredentialCountRequest{}
	response, err := h.client.GetCredentialCounts(context.Background(), request)
	if err != nil {
		h.logger.Error("failed-to-get-credential-counts", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	orgs := map[string]*Organization{}
	templateOrgs := []*Organization{}
	for _, r := range response.CredentialCounts {
		org, seen := orgs[r.Owner]
		if !seen {
			org = &Organization{
				Name:            r.Owner,
				CredentialCount: r.Count,
			}
			templateOrgs = append(templateOrgs, org)
			orgs[r.Owner] = org
		}

		org.CredentialCount += r.Count
		org.Repositories = append(org.Repositories, Repository{
			Name:            r.Repository,
			CredentialCount: r.Count,
		})
	}

	buf := bytes.NewBuffer([]byte{})
	err = h.template.Execute(buf, TemplateData{
		Organizations: templateOrgs,
	})

	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

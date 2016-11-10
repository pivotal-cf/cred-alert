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

type Organization struct {
	Name            string
	CredentialCount int64
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

	orgs := []*Organization{}
	for _, r := range response.CredentialCounts {
		orgs = append(orgs, &Organization{
			Name:            r.Owner,
			CredentialCount: r.Count,
		})
	}

	buf := bytes.NewBuffer([]byte{})
	err = h.template.Execute(buf, TemplateData{
		Organizations: orgs,
	})

	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

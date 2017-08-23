package api

import (
	"bytes"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	context "golang.org/x/net/context"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IndexRevokClient

type IndexRevokClient interface {
	GetCredentialCounts(ctx context.Context, in *revokpb.CredentialCountRequest, opts ...grpc.CallOption) (*revokpb.CredentialCountResponse, error)
}

type IndexHandler struct {
	logger   lager.Logger
	template *template.Template
	client   IndexRevokClient
}

func NewIndexHandler(
	logger lager.Logger,
	template *template.Template,
	client IndexRevokClient,
) *IndexHandler {
	return &IndexHandler{
		logger:   logger,
		template: template,
		client:   client,
	}
}

type Organization struct {
	Name            string
	CredentialCount int64
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	err = h.template.Execute(buf, orgs)

	if err != nil {
		h.logger.Error("failed-to-execute-template", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

package api

import (
	"bytes"
	"context"
	"cred-alert/revokpb"
	"crypto/tls"
	"crypto/x509"
	"html/template"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"
)

type indexHandler struct {
	logger        lager.Logger
	template      *template.Template
	rpcServerAddr string
	serverName    string
	clientCert    tls.Certificate
	rootCAs       *x509.CertPool
}

func NewIndexHandler(
	logger lager.Logger,
	template *template.Template,
	rpcServerAddr string,
	serverName string,
	clientCert tls.Certificate,
	rootCAs *x509.CertPool,
) http.Handler {
	return &indexHandler{
		logger:        logger,
		template:      template,
		rpcServerAddr: rpcServerAddr,
		serverName:    serverName,
		clientCert:    clientCert,
		rootCAs:       rootCAs,
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
	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   h.serverName,
		Certificates: []tls.Certificate{h.clientCert},
		RootCAs:      h.rootCAs,
	})

	dialOption := grpc.WithTransportCredentials(transportCreds)
	conn, err := grpc.Dial(h.rpcServerAddr, dialOption)
	if err != nil {
		h.logger.Error("dial-error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client := revokpb.NewRevokClient(conn)
	request := &revokpb.CredentialCountRequest{}
	response, err := client.GetCredentialCounts(context.Background(), request)
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

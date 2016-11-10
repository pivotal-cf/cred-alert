package api

import (
	"cred-alert/revok/web"
	"crypto/tls"
	"crypto/x509"
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func NewHandler(
	logger lager.Logger,
	template *template.Template,
	rpcServerAddr string,
	serverName string,
	clientCert tls.Certificate,
	rootCAs *x509.CertPool,
) (http.Handler, error) {
	indexHandler := NewIndexHandler(
		logger,
		template,
		rpcServerAddr,
		serverName,
		clientCert,
		rootCAs,
	)

	handlers := rata.Handlers{
		web.Index: indexHandler,
	}

	return rata.NewRouter(web.Routes, handlers)
}

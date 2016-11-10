package api

import (
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func NewHandler(
	logger lager.Logger,
	template *template.Template,
	client revokpb.RevokClient,
) http.Handler {
	indexHandler := NewIndexHandler(
		logger,
		template,
		client,
	)

	mux := http.NewServeMux()
	mux.Handle("/", indexHandler)

	return mux
}

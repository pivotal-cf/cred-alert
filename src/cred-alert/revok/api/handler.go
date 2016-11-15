package api

import (
	"cred-alert/revok/web"
	"cred-alert/revokpb"
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func NewHandler(
	logger lager.Logger,
	template *template.Template,
	client revokpb.RevokClient,
) (http.Handler, error) {
	indexHandler := NewIndexHandler(
		logger,
		template,
		client,
	)

	handlers := rata.Handlers{
		web.Index: indexHandler,
	}

	return rata.NewRouter(web.Routes, handlers)
}

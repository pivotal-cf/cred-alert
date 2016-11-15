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
	return rata.NewRouter(web.Routes, rata.Handlers{
		web.Index:        NewIndexHandler(logger, template, client),
		web.Organization: NewOrganizationHandler(logger, template, client),
	})
}

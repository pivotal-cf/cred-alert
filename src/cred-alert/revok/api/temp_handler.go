package api

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type tempHandler struct {
	logger lager.Logger
}

func NewTempHandler(
	logger lager.Logger,
) http.Handler {
	return &tempHandler{
		logger: logger,
	}
}

func (h *tempHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "Hi\n")
}

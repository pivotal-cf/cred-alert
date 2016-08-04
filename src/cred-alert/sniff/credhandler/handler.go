package credhandler

import (
	"cred-alert/scanners"

	"code.cloudfoundry.org/lager"
)

type Handler struct {
	handleFunc func(lager.Logger, scanners.Line) error
	credsFound int
}

func New(handleFunc func(lager.Logger, scanners.Line) error) *Handler {
	return &Handler{
		handleFunc: handleFunc,
	}
}

func (h *Handler) HandleViolation(logger lager.Logger, violation scanners.Line) error {
	h.credsFound++

	return h.handleFunc(logger, violation)
}

func (h *Handler) CredentialsFound() bool {
	return h.credsFound > 0
}

func (h *Handler) CredentialCount() int {
	return h.credsFound
}

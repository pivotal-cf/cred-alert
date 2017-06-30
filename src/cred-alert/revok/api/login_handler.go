package api

import (
	"encoding/base64"
	"net/http"

	"code.cloudfoundry.org/lager"

	"golang.org/x/oauth2"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const OAuthStateCookie = "_credential_count_publisher_oauth_state"

type loginHandler struct {
	logger       lager.Logger
	sessionStore sessions.Store
	config       oauth2.Config
}

func NewLoginHandler(logger lager.Logger, sessionStore sessions.Store, config oauth2.Config) http.Handler {
	return &loginHandler{
		logger:       logger,
		sessionStore: sessionStore,
		config:       config,
	}
}

func (h *loginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.New(r, OAuthStateCookie)
	if err != nil {
		h.logger.Error("login-new-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	randomToken := base64.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32))

	session.Values["state"] = randomToken
	err = h.sessionStore.Save(r, w, session)
	if err != nil {
		h.logger.Error("login-save-state-to-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authLoginURL := h.config.AuthCodeURL(randomToken)

	http.Redirect(w, r, authLoginURL, http.StatusFound)
}

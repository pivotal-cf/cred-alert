package middleware

import (
	"cred-alert/revok/api"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"golang.org/x/oauth2"

	"github.com/gorilla/sessions"
)

type authenticatedHandler struct {
	logger       lager.Logger
	sessionStore sessions.Store
	config       oauth2.Config
	authDomain   string
	handler      http.Handler
}

func NewAuthenticatedHandler(
	logger lager.Logger,
	sessionStore sessions.Store,
	config oauth2.Config,
	authDomain string,
	handler http.Handler) http.Handler {
	return &authenticatedHandler{
		logger:       logger,
		sessionStore: sessionStore,
		config:       config,
		authDomain:   authDomain,
		handler:      handler,
	}
}

func (h *authenticatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenSession, err := h.sessionStore.Get(r, api.OAuthTokenCookie)
	if err != nil {
		h.logger.Error("auth-handler-get-token-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	refreshTokenSession, err := h.sessionStore.Get(r, api.OAuthRefreshTokenCookie)
	if err != nil {
		h.logger.Error("auth-handler-get-refresh-token-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	accessTokenVal := tokenSession.Values["access_token"]
	expiryVal := tokenSession.Values["expiry"]
	tokenTypeVal := tokenSession.Values["token_type"]
	refreshTokenVal := refreshTokenSession.Values["refresh_token"]

	var accessToken string
	var ok bool
	if accessToken, ok = accessTokenVal.(string); !ok {
		err := fmt.Errorf("unable to get access token")
		h.logger.Error("auth-handler-failed-to-get-access-token", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	var expiry time.Time
	if expiry, ok = expiryVal.(time.Time); !ok {
		err := fmt.Errorf("unable to get token expiry")
		h.logger.Error("auth-handler-failed-to-get-token-expiry", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	var refreshToken string
	if refreshToken, ok = refreshTokenVal.(string); !ok {
		err := fmt.Errorf("unable to get refresh token")
		h.logger.Error("auth-handler-failed-to-get-refresh-token", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	var tokenType string
	if tokenType, ok = tokenTypeVal.(string); !ok {
		err := fmt.Errorf("unable to get token type")
		h.logger.Error("auth-handler-failed-to-get-token-type", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		Expiry:       expiry,
	}

	response, err := http.Get(fmt.Sprintf("%s/userinfo?access_token=%s", h.authDomain, token.AccessToken))
	if err != nil {
		h.logger.Error("auth-handler-user-info", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	if response.StatusCode > http.StatusBadRequest {
		err := fmt.Errorf("Unexpected response status code: %d", response.StatusCode)
		h.logger.Error("auth-handler-user-info", err)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	h.handler.ServeHTTP(w, r)
}

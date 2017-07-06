package api

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"

	"golang.org/x/oauth2"

	"github.com/gorilla/sessions"
)

const (
	OAuthTokenCookie        = "_credential_count_publisher_token"
	OAuthRefreshTokenCookie = "_credential_count_publisher_refresh_token"
)

func init() {
	gob.Register(time.Time{})
}

type oauthCallbackHandler struct {
	logger       lager.Logger
	sessionStore sessions.Store
	config       oauth2.Config
	authDomain   string
}

func NewOAuthCallbackHandler(
	logger lager.Logger,
	sessionStore sessions.Store,
	config oauth2.Config,
	authDomain string,
) http.Handler {
	return &oauthCallbackHandler{
		logger:       logger,
		sessionStore: sessionStore,
		config:       config,
		authDomain:   authDomain,
	}
}

func (h *oauthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.Get(r, OAuthStateCookie)
	if err != nil {
		h.logger.Error("oauth-callback-get-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cookieState := session.Values["state"]

	urlState := r.FormValue("state")
	if urlState != cookieState {
		err := fmt.Errorf("invalid oauth state, expected '%s', got '%s'\n", cookieState, urlState)
		h.logger.Error("oauth-callback-state-check", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := h.config.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("oauth-callback-code-exchange", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	response, err := http.Get(fmt.Sprintf("%s/userinfo?access_token=%s", h.authDomain, token.AccessToken))
	if err != nil {
		h.logger.Error("oauth-callback-user-info", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if response.StatusCode > http.StatusBadRequest {
		err := fmt.Errorf("Unexpected response status code: %d", response.StatusCode)
		h.logger.Error("oauth-callback-user-info", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	defer response.Body.Close()

	// fmt.Printf("access token length: %d\n", len(token.AccessToken))
	// fmt.Printf("access token: '%s'\n", token.AccessToken)

	// fmt.Printf("token type: '%s'\n", token.TokenType)

	// fmt.Printf("refresh token length: %d\n", len(token.RefreshToken))
	// fmt.Printf("refresh token: '%s'\n", token.RefreshToken)

	// fmt.Printf("expiry: '%v'\n", token.Expiry)

	tokenSession, err := h.sessionStore.New(r, OAuthTokenCookie)
	if err != nil {
		h.logger.Error("oauth-callback-get-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	refreshTokenSession, err := h.sessionStore.New(r, OAuthRefreshTokenCookie)
	if err != nil {
		h.logger.Error("oauth-callback-get-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tokenSession.Values["access_token"] = token.AccessToken
	tokenSession.Values["expiry"] = token.Expiry
	tokenSession.Values["token_type"] = token.TokenType
	refreshTokenSession.Values["refresh_token"] = token.RefreshToken

	err = h.sessionStore.Save(r, w, tokenSession)
	if err != nil {
		h.logger.Error("oauth-callback-save-token-to-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.sessionStore.Save(r, w, refreshTokenSession)
	if err != nil {
		h.logger.Error("oauth-callback-save-refresh-token-to-session", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

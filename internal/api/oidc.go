package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/nikiv/ansible-ui/internal/config"
	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	oidcStateCookie    = "aui_oidc_state"
	oidcVerifierCookie = "aui_oidc_verifier" // PKCE code_verifier (per-flow)
)

type oidcAuth struct {
	verifier      *oidc.IDTokenVerifier
	oauth2        oauth2.Config
	usernameClaim string
	defaultRole   string
	endSession    string // RP-initiated logout endpoint (from discovery, may be empty)
}

func newOIDC(ctx context.Context, cfg config.OIDCConfig) (*oidcAuth, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	username := cfg.UsernameClaim
	if username == "" {
		username = "preferred_username"
	}
	role := cfg.DefaultRole
	if role == "" {
		role = "user"
	}
	// Discover the (optional) RP-initiated logout endpoint.
	var disc struct {
		EndSession string `json:"end_session_endpoint"`
	}
	_ = provider.Claims(&disc)
	return &oidcAuth{
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		usernameClaim: username,
		defaultRole:   role,
		endSession:    disc.EndSession,
	}, nil
}

// handleOIDCLogin starts the auth-code flow with PKCE (redirects to the IdP).
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErr(w, http.StatusNotFound, "oidc not enabled")
		return
	}
	state := randHex(16)
	verifier := oauth2.GenerateVerifier()
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookie, Value: state, Path: "/", MaxAge: 600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
	http.SetCookie(w, &http.Cookie{
		Name: oidcVerifierCookie, Value: verifier, Path: "/", MaxAge: 600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
	url := s.oidc.oauth2.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

// handleOIDCCallback completes the flow: verify state + PKCE + id_token, apply a
// required-claim restriction, link by email, upsert + sign in.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErr(w, http.StatusNotFound, "oidc not enabled")
		return
	}
	st, err := r.Cookie(oidcStateCookie)
	if err != nil || st.Value == "" || st.Value != r.URL.Query().Get("state") {
		writeErr(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	var opts []oauth2.AuthCodeOption
	if v, verr := r.Cookie(oidcVerifierCookie); verr == nil && v.Value != "" {
		opts = append(opts, oauth2.VerifierOption(v.Value)) // PKCE
	}
	tok, err := s.oidc.oauth2.Exchange(r.Context(), r.URL.Query().Get("code"), opts...)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "token exchange failed: "+err.Error())
		return
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		writeErr(w, http.StatusBadGateway, "no id_token in response")
		return
	}
	idToken, err := s.oidc.verifier.Verify(r.Context(), rawID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "id_token verification failed: "+err.Error())
		return
	}
	var claims map[string]any
	_ = idToken.Claims(&claims)

	// Restrict login by a configured claim (e.g. hd=example.com, groups=eng).
	if rule := s.oidcRequiredClaim(r.Context()); rule != "" && !claimSatisfies(claims, rule) {
		s.log.Warn("oidc login rejected — required claim not satisfied", "rule", rule, "sub", idToken.Subject)
		http.Redirect(w, r, "/?sso_error=access_denied", http.StatusFound)
		return
	}

	username := strClaim(claims, s.oidc.usernameClaim)
	if username == "" {
		username = strClaim(claims, "email")
	}
	if username == "" {
		username = idToken.Subject
	}
	email := strClaim(claims, "email")

	// Account linking: if an existing account already has this email, sign in as
	// that user (link the external identity) instead of creating a duplicate.
	var u *model.User
	if email != "" {
		if existing, gerr := s.store.GetUserByEmail(r.Context(), email); gerr == nil && existing != nil {
			u = existing
		}
	}
	if u == nil {
		u, err = s.store.UpsertExternalUser(r.Context(), username, email, s.oidc.defaultRole)
		if err != nil {
			s.handleStoreErr(w, err)
			return
		}
	}
	// Group → role mapping from the configured groups claim.
	s.applyGroupRole(r.Context(), u, claimStrings(claims, s.oidcGroupsClaim(r.Context())), s.oidc.defaultRole)
	s.issueSession(w, r, u)
	s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "oidc")
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleOIDCLogout clears the session and, if the IdP supports RP-initiated
// logout, redirects there (so the SSO session ends too); otherwise to "/".
func (s *Server) handleOIDCLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
	dest := "/"
	if s.oidc != nil && s.oidc.endSession != "" {
		dest = s.oidc.endSession
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func strClaim(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

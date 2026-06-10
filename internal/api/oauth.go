package api

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/nikiv/ansible-ui/internal/config"
)

const oauthStateCookie = "aui_oauth_state"

// oauthProvider is a plain OAuth2 sign-in provider (GitHub / Bitbucket): the
// auth-code flow plus a userinfo fetch that yields a username + email, which we
// upsert as a local account (same as LDAP/OIDC external users).
type oauthProvider struct {
	name        string
	oauth2      oauth2.Config
	userInfoURL string
	role        string
	parseUser   func(map[string]any) (username, email string)
}

func newGitHub(c config.OAuthConfig, role string) *oauthProvider {
	return &oauthProvider{
		name: "github",
		oauth2: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		},
		userInfoURL: "https://api.github.com/user",
		role:        role,
		parseUser: func(m map[string]any) (string, string) {
			return str(m["login"]), str(m["email"])
		},
	}
}

func newBitbucket(c config.OAuthConfig, role string) *oauthProvider {
	return &oauthProvider{
		name: "bitbucket",
		oauth2: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Scopes:       []string{"account"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
				TokenURL: "https://bitbucket.org/site/oauth2/access_token",
			},
		},
		userInfoURL: "https://api.bitbucket.org/2.0/user",
		role:        role,
		parseUser: func(m map[string]any) (string, string) {
			return str(m["username"]), str(m["email"]) // bitbucket: email needs a separate call; username is enough
		},
	}
}

// oauthLogin starts the auth-code flow for the given provider.
func (s *Server) oauthLogin(w http.ResponseWriter, r *http.Request, p *oauthProvider) {
	if p == nil {
		writeErr(w, http.StatusNotFound, "provider not enabled")
		return
	}
	state := p.name + ":" + randHex(16)
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: state, Path: "/", MaxAge: 600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
	http.Redirect(w, r, p.oauth2.AuthCodeURL(state), http.StatusFound)
}

// oauthCallback completes the flow: verify state, exchange the code, fetch the
// user, then upsert + issue a session.
func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request, p *oauthProvider) {
	if p == nil {
		writeErr(w, http.StatusNotFound, "provider not enabled")
		return
	}
	st, err := r.Cookie(oauthStateCookie)
	if err != nil || st.Value == "" || st.Value != r.URL.Query().Get("state") {
		writeErr(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	tok, err := p.oauth2.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "token exchange failed: "+err.Error())
		return
	}
	info, err := fetchUserInfo(r.Context(), p, tok)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "userinfo failed: "+err.Error())
		return
	}
	username, email := p.parseUser(info)
	if username == "" {
		writeErr(w, http.StatusBadGateway, "no username from "+p.name)
		return
	}
	u, err := s.store.UpsertExternalUser(r.Context(), username, email, p.role)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.issueSession(w, r, u)
	s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, p.name)
	http.Redirect(w, r, "/", http.StatusFound)
}

func fetchUserInfo(ctx context.Context, p *oauthProvider, tok *oauth2.Token) (map[string]any, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := p.oauth2.Client(ctx, tok).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// route handlers
func (s *Server) handleGitHubLogin(w http.ResponseWriter, r *http.Request)    { s.oauthLogin(w, r, s.github) }
func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) { s.oauthCallback(w, r, s.github) }
func (s *Server) handleBitbucketLogin(w http.ResponseWriter, r *http.Request) {
	s.oauthLogin(w, r, s.bitbucket)
}
func (s *Server) handleBitbucketCallback(w http.ResponseWriter, r *http.Request) {
	s.oauthCallback(w, r, s.bitbucket)
}

package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/nikiv/ansible-ui/internal/model"
)

const sessionCookie = "aui_session"

type ctxKey int

const userKey ctxKey = 0

// currentUser returns the authenticated user attached by authGuard, or nil.
func currentUser(r *http.Request) *model.User {
	u, _ := r.Context().Value(userKey).(*model.User)
	return u
}

func isPublicPath(method, path string) bool {
	switch path {
	case "/api/health", "/api/auth/status":
		return true
	case "/api/auth/oidc/login", "/api/auth/oidc/callback", "/api/auth/oidc/logout",
		"/api/auth/saml/metadata", "/api/auth/saml/login",
		"/api/auth/github/login", "/api/auth/github/callback",
		"/api/auth/bitbucket/login", "/api/auth/bitbucket/callback":
		return method == http.MethodGet
	case "/api/auth/saml/acs":
		return method == http.MethodPost // IdP POSTs the SAMLResponse here
	case "/api/auth/setup", "/api/auth/login":
		return method == http.MethodPost
	}
	return false
}

// authGuard requires a valid session for every /api and /ws endpoint except the
// public ones. Static assets and SPA routes are always served (the app must
// load to render the login screen).
func (s *Server) authGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/ws/") {
			next.ServeHTTP(w, r)
			return
		}
		// Inbound webhooks are authorised by the token in the URL.
		if strings.HasPrefix(p, "/api/webhooks/") {
			next.ServeHTTP(w, r)
			return
		}
		// Runner agents authenticate with the shared runner token (X-Runner-Token),
		// not a user session — register + heartbeat bypass the session guard.
		if r.Method == http.MethodPost && strings.HasPrefix(p, "/api/runners/") &&
			(p == "/api/runners/register" || strings.HasSuffix(p, "/heartbeat")) {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicPath(r.Method, p) {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if u, serr := s.store.SessionUser(r.Context(), c.Value); serr == nil {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
				return
			}
		}
		// Bearer API token (CI / automation).
		if tok := bearerToken(r); tok != "" {
			if u, tid, terr := s.store.TokenUser(r.Context(), hashToken(tok)); terr == nil {
				go s.store.TouchToken(context.Background(), tid)
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
				return
			}
		}
		writeErr(w, http.StatusUnauthorized, "authentication required")
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := currentUser(r)
	if u == nil || u.Role != model.RoleAdmin {
		writeErr(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

// ---- handlers ----------------------------------------------------------

// handleAuthStatus tells the SPA whether to show setup, login, or the app.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var user *model.User
	if c, cerr := r.Cookie(sessionCookie); cerr == nil && c.Value != "" {
		user, _ = s.store.SessionUser(r.Context(), c.Value)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"needsSetup":    n == 0,
		"authenticated": user != nil,
		"user":          user,
		"providers": map[string]bool{
			"local":     true,
			"ldap":      s.cfg.LDAP.Enabled,
			"oidc":      s.oidc != nil,
			"saml":      s.saml != nil,
			"github":    s.github != nil,
			"bitbucket": s.bitbucket != nil,
		},
		// SSO auto-login: the SPA redirects unauthenticated users straight to the
		// IdP when this is on (and OIDC is the configured provider).
		"oidcAutoLogin": s.oidc != nil && s.oidcAutoLogin(r.Context()),
		"docsEnabled":   s.effectiveDocsEnabled(r),
	})
}

// handleSetup creates the first admin (only allowed when no users exist).
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if n > 0 {
		writeErr(w, http.StatusConflict, "already initialised")
		return
	}
	var in struct{ Username, Email, Password string }
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(in.Username) < 2 || len(in.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "username ≥ 2 and password ≥ 6 characters")
		return
	}
	u := &model.User{Username: in.Username, Email: in.Email, Role: model.RoleAdmin}
	if err := s.createUserWithPassword(r.Context(), u, in.Password); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.issueSession(w, r, u)
	s.recordActivity(r.Context(), u.Username, "auth.setup", u.Username, "")
	writeJSON(w, http.StatusCreated, u)
}

// handleLogin verifies credentials and starts a session.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password, Code string }
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	username := strings.TrimSpace(in.Username)

	// 1) Local account.
	if u, hash, err := s.store.GetUserByUsername(r.Context(), username); err == nil {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) == nil {
			// Two-factor gate: when enabled, the password is not enough — a valid
			// TOTP code is required before a session is issued.
			if u.TwoFactorEnabled {
				if in.Code == "" {
					writeJSON(w, http.StatusOK, map[string]bool{"twoFactorRequired": true})
					return
				}
				blob, _, terr := s.store.GetUserTOTP(r.Context(), u.ID)
				secret, derr := []byte(nil), error(nil)
				if terr == nil && len(blob) > 0 {
					secret, derr = s.cipher.Decrypt(blob)
				}
				if terr != nil || derr != nil || !validateTOTP(string(secret), in.Code) {
					writeErr(w, http.StatusUnauthorized, "incorrect 2FA code")
					return
				}
			} else if u.EmailOTPEnabled {
				// Email second factor: first POST (no code) mails a single-use code;
				// the client re-submits username/password + code.
				if in.Code == "" {
					code := newOTPCode()
					_ = s.store.SetEmailOTP(r.Context(), u.ID, hashOTP(code), time.Now().Add(10*time.Minute))
					if err := s.sendOTPEmail(r.Context(), u.Email, code); err != nil {
						s.log.Warn("email-otp send failed", "user", u.Username, "err", err)
						writeErr(w, http.StatusBadGateway, "could not send the login code email")
						return
					}
					writeJSON(w, http.StatusOK, map[string]bool{"emailOtpRequired": true})
					return
				}
				ok, cerr := s.store.ConsumeEmailOTP(r.Context(), u.ID, hashOTP(in.Code))
				if cerr != nil || !ok {
					writeErr(w, http.StatusUnauthorized, "incorrect or expired login code")
					return
				}
			}
			s.issueSession(w, r, u)
			s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "local")
			writeJSON(w, http.StatusOK, u)
			return
		}
	}

	// 2) LDAP / directory. Local accounts are tried first (above), so a directory
	// outage never locks out a local admin — this is the local-auth fallback.
	if s.cfg.LDAP.Enabled {
		if u, lerr := s.authenticateLDAP(r.Context(), username, in.Password); lerr == nil {
			s.issueSession(w, r, u)
			s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "ldap")
			writeJSON(w, http.StatusOK, u)
			return
		} else if errors.Is(lerr, errLDAPUnavailable) {
			// Connectivity/config failure (not a bad password) — note that local
			// accounts remain usable so the operator can recover.
			s.log.Warn("ldap unavailable — local accounts can still sign in (fallback)", "user", username, "err", lerr)
		} else {
			s.log.Debug("ldap auth failed", "user", username, "err", lerr)
		}
	}

	// 3) RADIUS (RFC 2865, PAP). Also after local, so a RADIUS outage never locks
	// out a local admin.
	if s.cfg.RADIUS.Enabled {
		if u, rerr := s.authenticateRADIUS(r.Context(), username, in.Password); rerr == nil {
			s.issueSession(w, r, u)
			s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "radius")
			writeJSON(w, http.StatusOK, u)
			return
		} else if errors.Is(rerr, errRADIUSUnavailable) {
			s.log.Warn("radius unavailable — local accounts can still sign in (fallback)", "user", username, "err", rerr)
		} else {
			s.log.Debug("radius auth failed", "user", username, "err", rerr)
		}
	}

	// 4) TACACS+ (RFC 8907, ASCII). Also after local.
	if s.cfg.TACACS.Enabled {
		if u, terr := s.authenticateTACACS(r.Context(), username, in.Password); terr == nil {
			s.issueSession(w, r, u)
			s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "tacacs")
			writeJSON(w, http.StatusOK, u)
			return
		} else if errors.Is(terr, errTACACSUnavailable) {
			s.log.Warn("tacacs+ unavailable — local accounts can still sign in (fallback)", "user", username, "err", terr)
		} else {
			s.log.Debug("tacacs+ auth failed", "user", username, "err", terr)
		}
	}

	writeErr(w, http.StatusUnauthorized, "invalid username or password")
}

// handleLogout destroys the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if u := currentUser(r); u != nil {
		s.recordActivity(r.Context(), u.Username, "auth.logout", u.Username, "")
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the current user.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- user management (admin) ------------------------------------------

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		Username, Email, Password, Role string
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(in.Username) < 2 || len(in.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "username ≥ 2 and password ≥ 6 characters")
		return
	}
	role := in.Role
	if role != model.RoleAdmin {
		role = model.RoleUser
	}
	u := &model.User{Username: in.Username, Email: in.Email, Role: role}
	if err := s.createUserWithPassword(r.Context(), u, in.Password); err != nil {
		writeErr(w, http.StatusConflict, "could not create user (username taken?)")
		return
	}
	s.recordActivity(r.Context(), "", "user.created", u.Username, u.Role)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if me := currentUser(r); me != nil && me.ID == id {
		writeErr(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	name := id
	if u, err := s.store.GetUser(r.Context(), id); err == nil {
		name = u.Username
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "user.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers -----------------------------------------------------------

func (s *Server) createUserWithPassword(ctx context.Context, u *model.User, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.CreateUser(ctx, u, string(hash))
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, u *model.User) {
	tok := make([]byte, 32)
	_, _ = rand.Read(tok)
	token := hex.EncodeToString(tok)
	expires := time.Now().Add(time.Duration(s.cfg.SessionTTL) * time.Hour)
	if err := s.store.CreateSession(r.Context(), token, u.ID, expires); err != nil {
		s.log.Error("create session failed", "err", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
}

package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"

	"github.com/nikiv/ansible-ui/internal/model"
)

// errLDAPUnavailable marks a directory connectivity/configuration failure (dial,
// StartTLS, service bind, search) — as opposed to a genuine auth rejection. The
// login handler treats it as "directory unreachable", so local accounts still
// sign in (local auth is tried first, making this a true fallback).
var errLDAPUnavailable = errors.New("ldap unavailable")

// authenticateLDAP verifies a username/password against the configured directory
// (bind-as-service → search → bind-as-user) and upserts a local user row. When
// the ldap.debug setting is on, each step is logged so admins can diagnose
// directory problems without raising the global log level.
func (s *Server) authenticateLDAP(ctx context.Context, username, password string) (*model.User, error) {
	cfg := s.cfg.LDAP
	debug := s.ldapDebug(ctx)
	dbg := func(msg string, args ...any) {
		if debug {
			s.log.Info("[ldap] "+msg, args...)
		}
	}
	if cfg.URL == "" || cfg.UserBaseDN == "" {
		return nil, fmt.Errorf("%w: not configured (need LDAP_URL + LDAP_USER_BASE_DN)", errLDAPUnavailable)
	}
	if password == "" {
		return nil, errors.New("empty password")
	}
	dbg("dialing", "url", cfg.URL, "startTLS", cfg.StartTLS, "user", username)

	conn, err := ldap.DialURL(cfg.URL)
	if err != nil {
		dbg("dial failed", "url", cfg.URL, "err", err)
		return nil, fmt.Errorf("%w: dial %s: %v", errLDAPUnavailable, cfg.URL, err)
	}
	defer conn.Close()

	if cfg.StartTLS {
		if err := conn.StartTLS(&tls.Config{ServerName: hostOnly(cfg.URL)}); err != nil {
			dbg("starttls failed", "err", err)
			return nil, fmt.Errorf("%w: starttls: %v", errLDAPUnavailable, err)
		}
	}

	// Bind as the service account (or anonymously) to search.
	if cfg.BindDN != "" {
		dbg("service bind", "bindDN", cfg.BindDN)
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			dbg("service bind failed", "bindDN", cfg.BindDN, "err", err)
			return nil, fmt.Errorf("%w: service bind %s: %v", errLDAPUnavailable, cfg.BindDN, err)
		}
	}

	groupAttr := s.ldapGroupAttr(ctx)
	filter := fmt.Sprintf(cfg.UserFilter, ldap.EscapeFilter(username))
	dbg("search", "baseDN", cfg.UserBaseDN, "filter", filter, "groupAttr", groupAttr)
	res, err := conn.Search(ldap.NewSearchRequest(
		cfg.UserBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 10, false,
		filter, []string{"dn", cfg.EmailAttr, groupAttr}, nil,
	))
	if err != nil {
		dbg("search failed", "err", err)
		return nil, fmt.Errorf("%w: search: %v", errLDAPUnavailable, err)
	}
	if len(res.Entries) != 1 {
		dbg("user not uniquely found", "entries", len(res.Entries))
		return nil, errors.New("user not found in directory")
	}
	entry := res.Entries[0]
	dbg("user found", "dn", entry.DN)

	// Verify the password by binding as the user.
	if err := conn.Bind(entry.DN, password); err != nil {
		dbg("user bind failed (bad password)", "dn", entry.DN)
		return nil, errors.New("invalid directory credentials")
	}
	dbg("user bind ok", "dn", entry.DN)

	role := cfg.DefaultRole
	if role == "" {
		role = model.RoleUser
	}
	u, err := s.store.UpsertExternalUser(ctx, username, entry.GetAttributeValue(cfg.EmailAttr), role)
	if err != nil {
		return nil, err
	}
	// Group → role mapping (directory groups are authoritative).
	s.applyGroupRole(ctx, u, entry.GetAttributeValues(groupAttr), role)
	return u, nil
}

func hostOnly(url string) string {
	u := url
	for _, p := range []string{"ldaps://", "ldap://"} {
		if len(u) >= len(p) && u[:len(p)] == p {
			u = u[len(p):]
			break
		}
	}
	for i := 0; i < len(u); i++ {
		if u[i] == ':' || u[i] == '/' {
			return u[:i]
		}
	}
	return u
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	settingGroupRoleRules  = "auth.group_role_rules"  // JSON [{group,role}]
	settingLDAPGroupAttr   = "auth.ldap_group_attr"   // default memberOf
	settingOIDCGroupsClaim = "auth.oidc_groups_claim" // default groups
	settingLDAPDebug       = "auth.ldap_debug"        // verbose LDAP login logging
	settingOIDCRequiredClaim = "auth.oidc_required_claim" // e.g. "hd=example.com" — restrict login by a claim
	settingOIDCAutoLogin     = "auth.oidc_auto_login"     // auto-redirect unauthenticated users to the IdP
)

type groupRule struct {
	Group string `json:"group"`
	Role  string `json:"role"`
}

func roleRank(role string) int {
	switch role {
	case model.RoleAdmin:
		return 2
	case model.RoleUser:
		return 1
	}
	return 0
}

// mapGroupsToRole returns the highest-privilege role whose rule group matches any
// of the user's groups (case-insensitive substring, so "admins" matches
// "cn=admins,ou=groups,…"); falls back to defaultRole when nothing matches.
func mapGroupsToRole(groups []string, rules []groupRule, defaultRole string) string {
	best, bestRank := "", -1
	for _, r := range rules {
		needle := strings.ToLower(strings.TrimSpace(r.Group))
		if needle == "" {
			continue
		}
		for _, g := range groups {
			if strings.Contains(strings.ToLower(g), needle) {
				if rk := roleRank(r.Role); rk > bestRank {
					best, bestRank = r.Role, rk
				}
				break
			}
		}
	}
	if best != "" {
		return best
	}
	return defaultRole
}

// claimStrings extracts a string slice from an OIDC claim that may be a []any of
// strings or a single string.
func claimStrings(claims map[string]any, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		return []string{t}
	}
	return nil
}

func (s *Server) groupRoleRules(ctx context.Context) []groupRule {
	v, _, _ := s.store.GetSetting(ctx, settingGroupRoleRules)
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var rules []groupRule
	_ = json.Unmarshal([]byte(v), &rules)
	return rules
}

func (s *Server) ldapGroupAttr(ctx context.Context) string {
	if v, _, _ := s.store.GetSetting(ctx, settingLDAPGroupAttr); strings.TrimSpace(v) != "" {
		return v
	}
	return "memberOf"
}

func (s *Server) oidcGroupsClaim(ctx context.Context) string {
	if v, _, _ := s.store.GetSetting(ctx, settingOIDCGroupsClaim); strings.TrimSpace(v) != "" {
		return v
	}
	return "groups"
}

// ldapDebug reports whether verbose LDAP login logging is enabled (admins turn it
// on to diagnose directory bind/search/connectivity problems).
func (s *Server) ldapDebug(ctx context.Context) bool {
	return s.boolSetting(ctx, settingLDAPDebug)
}

// oidcRequiredClaim returns the "key=value" claim an OIDC user must satisfy to
// log in (empty = no restriction).
func (s *Server) oidcRequiredClaim(ctx context.Context) string {
	v, _, _ := s.store.GetSetting(ctx, settingOIDCRequiredClaim)
	return strings.TrimSpace(v)
}

// oidcAutoLogin reports whether unauthenticated users should be auto-redirected
// to the OIDC provider (SSO-only / kiosk style).
func (s *Server) oidcAutoLogin(ctx context.Context) bool {
	return s.boolSetting(ctx, settingOIDCAutoLogin)
}

// claimSatisfies reports whether claims satisfy a "key" / "key=value" rule. A
// bare key requires the claim be present + truthy; "key=value" matches a string
// claim equal to value, or an array claim (groups, etc.) that contains it.
func claimSatisfies(claims map[string]any, rule string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return true
	}
	key, want, hasEq := rule, "", false
	if i := strings.Index(rule, "="); i >= 0 {
		key, want, hasEq = strings.TrimSpace(rule[:i]), strings.TrimSpace(rule[i+1:]), true
	}
	v, ok := claims[key]
	if !ok {
		return false
	}
	if !hasEq {
		switch t := v.(type) {
		case string:
			return t != ""
		case bool:
			return t
		default:
			return v != nil
		}
	}
	if sv, ok := v.(string); ok {
		return sv == want
	}
	for _, item := range claimStrings(claims, key) {
		if item == want {
			return true
		}
	}
	return false
}

// applyGroupRole maps an external (LDAP/OIDC) user's groups to a role and updates
// the stored role when it differs — so directory group membership stays authoritative.
func (s *Server) applyGroupRole(ctx context.Context, u *model.User, groups []string, defaultRole string) {
	rules := s.groupRoleRules(ctx)
	if len(rules) == 0 || len(groups) == 0 {
		return
	}
	role := mapGroupsToRole(groups, rules, defaultRole)
	if role != "" && role != u.Role {
		if err := s.store.SetUserRole(ctx, u.ID, role); err == nil {
			u.Role = role
		}
	}
}

type authMappingConfig struct {
	Rules             []groupRule `json:"rules"`
	LDAPAttr          string      `json:"ldapGroupAttr"`
	OIDCClaim         string      `json:"oidcGroupsClaim"`
	LDAPDebug         bool        `json:"ldapDebug"`
	OIDCRequiredClaim string      `json:"oidcRequiredClaim"`
	OIDCAutoLogin     bool        `json:"oidcAutoLogin"`
}

func (s *Server) handleGetAuthMapping(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, authMappingConfig{
		Rules: s.groupRoleRules(r.Context()), LDAPAttr: s.ldapGroupAttr(r.Context()),
		OIDCClaim: s.oidcGroupsClaim(r.Context()), LDAPDebug: s.ldapDebug(r.Context()),
		OIDCRequiredClaim: s.oidcRequiredClaim(r.Context()), OIDCAutoLogin: s.oidcAutoLogin(r.Context()),
	})
}

func (s *Server) handleSetAuthMapping(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in authMappingConfig
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	b, _ := json.Marshal(in.Rules)
	_ = s.store.SetSetting(r.Context(), settingGroupRoleRules, string(b))
	_ = s.store.SetSetting(r.Context(), settingLDAPGroupAttr, strings.TrimSpace(in.LDAPAttr))
	_ = s.store.SetSetting(r.Context(), settingOIDCGroupsClaim, strings.TrimSpace(in.OIDCClaim))
	_ = s.store.SetSetting(r.Context(), settingLDAPDebug, boolStr(in.LDAPDebug))
	_ = s.store.SetSetting(r.Context(), settingOIDCRequiredClaim, strings.TrimSpace(in.OIDCRequiredClaim))
	_ = s.store.SetSetting(r.Context(), settingOIDCAutoLogin, boolStr(in.OIDCAutoLogin))
	s.recordActivity(r.Context(), s.actor(r.Context()), "auth.mappingUpdated", "", "")
	writeJSON(w, http.StatusOK, in)
}

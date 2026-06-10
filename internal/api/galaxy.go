package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/nikiv/ansible-ui/internal/notify"
)

// Private Ansible Galaxy / Automation Hub auth: an admin sets a galaxy server
// URL + token (+ optional extra `ansible-galaxy` CLI args). At run time the
// token is injected as ANSIBLE_GALAXY_SERVER_* env so the runner's
// `ansible-galaxy install -r requirements.yml` authenticates to the private server.

const (
	settingGalaxyURL     = "galaxy.server_url"
	settingGalaxyToken   = "galaxy.token"
	settingGalaxyCLIArgs = "galaxy.cli_args"
	settingNotifyProxy   = "notify.proxy_url"
)

// handleGetNotifyProxy / handleSetNotifyProxy manage the outbound proxy used for
// notification egress (alert-proxy: route chat/webhook alerts through a corporate
// proxy in restricted networks). http/https/socks5 URLs are accepted.
func (s *Server) handleGetNotifyProxy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	v, _, _ := s.store.GetSetting(r.Context(), settingNotifyProxy)
	writeJSON(w, http.StatusOK, map[string]string{"proxyUrl": v})
}

func (s *Server) handleSetNotifyProxy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		ProxyURL string `json:"proxyUrl"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	in.ProxyURL = strings.TrimSpace(in.ProxyURL)
	if err := notify.SetProxy(in.ProxyURL); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid proxy url: "+err.Error())
		return
	}
	_ = s.store.SetSetting(r.Context(), settingNotifyProxy, in.ProxyURL)
	s.recordActivity(r.Context(), s.actor(r.Context()), "notify.proxyUpdated", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"proxyUrl": in.ProxyURL})
}

type galaxyConfig struct {
	ServerURL string `json:"serverUrl"`
	Token     string `json:"token"`
	CLIArgs   string `json:"cliArgs"`
}

func (s *Server) handleGetGalaxy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	get := func(k string) string { v, _, _ := s.store.GetSetting(r.Context(), k); return v }
	writeJSON(w, http.StatusOK, galaxyConfig{
		ServerURL: get(settingGalaxyURL), Token: get(settingGalaxyToken), CLIArgs: get(settingGalaxyCLIArgs),
	})
}

func (s *Server) handleSetGalaxy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in galaxyConfig
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	_ = s.store.SetSetting(r.Context(), settingGalaxyURL, strings.TrimSpace(in.ServerURL))
	_ = s.store.SetSetting(r.Context(), settingGalaxyToken, strings.TrimSpace(in.Token))
	_ = s.store.SetSetting(r.Context(), settingGalaxyCLIArgs, strings.TrimSpace(in.CLIArgs))
	s.recordActivity(r.Context(), s.actor(r.Context()), "galaxy.settingsUpdated", "", "")
	writeJSON(w, http.StatusOK, in)
}

// galaxyEnv returns the ANSIBLE_GALAXY_SERVER_* environment for a run's
// `ansible-galaxy install`, so a private galaxy/Automation Hub authenticates.
// Empty when no server is configured.
func (s *Server) galaxyEnv(ctx context.Context) map[string]string {
	url, _, _ := s.store.GetSetting(ctx, settingGalaxyURL)
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	env := map[string]string{
		"ANSIBLE_GALAXY_SERVER_LIST":           "ansible_ui",
		"ANSIBLE_GALAXY_SERVER_ANSIBLE_UI_URL": url,
	}
	if tok, _, _ := s.store.GetSetting(ctx, settingGalaxyToken); strings.TrimSpace(tok) != "" {
		env["ANSIBLE_GALAXY_SERVER_ANSIBLE_UI_TOKEN"] = strings.TrimSpace(tok)
	}
	return env
}

// galaxyArgs returns the configured extra `ansible-galaxy` CLI args (split on
// whitespace), e.g. "--ignore-certs --timeout 60".
func (s *Server) galaxyArgs(ctx context.Context) []string {
	v, _, _ := s.store.GetSetting(ctx, settingGalaxyCLIArgs)
	return strings.Fields(strings.TrimSpace(v))
}

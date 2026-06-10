package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// runnerTokenOK validates the shared runner token. When no token is configured
// registration is open (dev convenience); in production set RUNNER_TOKEN.
func (s *Server) runnerTokenOK(r *http.Request) bool {
	if s.cfg.RunnerToken == "" {
		return true
	}
	return r.Header.Get("X-Runner-Token") == s.cfg.RunnerToken
}

// runnerStatus derives online/offline from the last heartbeat (built-in is
// always online — it ships with the control plane).
func runnerStatus(rn *model.Runner) string {
	if rn.Builtin {
		return "online"
	}
	if rn.LastSeenAt != nil && time.Since(*rn.LastSeenAt) < model.RunnerOnlineWindow {
		return "online"
	}
	return "offline"
}

// handleRegisterRunner upserts a runner keyed by its URL (token-authenticated).
func (s *Server) handleRegisterRunner(w http.ResponseWriter, r *http.Request) {
	if !s.runnerTokenOK(r) {
		writeErr(w, http.StatusUnauthorized, "invalid runner token")
		return
	}
	var in struct {
		Name          string   `json:"name"`
		URL           string   `json:"url"`
		Tags          []string `json:"tags"`
		Platform      string   `json:"platform"`
		Version       string   `json:"version"`
		MaxConcurrent int      `json:"maxConcurrent"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}
	rn := &model.Runner{Name: in.Name, URL: in.URL, Tags: in.Tags, Platform: in.Platform, Version: in.Version, MaxConcurrent: in.MaxConcurrent}
	if err := s.store.UpsertRunner(r.Context(), rn); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), "runner", "runner.registered", rn.Name, rn.URL)
	s.events.Publish(map[string]any{"type": "runners"})
	writeJSON(w, http.StatusOK, map[string]any{"id": rn.ID})
}

// handleRunnerHeartbeat refreshes a runner's last-seen time (token-authenticated).
func (s *Server) handleRunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.runnerTokenOK(r) {
		writeErr(w, http.StatusUnauthorized, "invalid runner token")
		return
	}
	if err := s.store.HeartbeatRunner(r.Context(), r.PathValue("id")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListRunners lists registered runners with derived online/offline status.
func (s *Server) handleListRunners(w http.ResponseWriter, r *http.Request) {
	runners, err := s.store.ListRunners(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]*model.Runner, 0, len(runners))
	for _, rn := range runners {
		rn.Status = runnerStatus(rn)
		out = append(out, rn)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDeleteRunner removes a runner from the registry (admin; not the built-in).
func (s *Server) handleDeleteRunner(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rn, err := s.store.GetRunner(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if rn.Builtin {
		writeErr(w, http.StatusBadRequest, "the built-in runner cannot be removed")
		return
	}
	if err := s.store.DeleteRunner(r.Context(), rn.ID); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.events.Publish(map[string]any{"type": "runners"})
	w.WriteHeader(http.StatusNoContent)
}

// runnerHTTPBase converts a runner's WebSocket base (ws://host:port) into its
// HTTP base (http://host:port) for the git-sync endpoint.
func runnerHTTPBase(wsURL string) string {
	switch {
	case strings.HasPrefix(wsURL, "wss://"):
		return "https://" + strings.TrimPrefix(wsURL, "wss://")
	case strings.HasPrefix(wsURL, "ws://"):
		return "http://" + strings.TrimPrefix(wsURL, "ws://")
	}
	return wsURL
}

// seedBuiltinRunner registers the control plane's configured runner so it shows
// up in the registry and can be targeted by the "default" tag.
func (s *Server) seedBuiltinRunner(ctx context.Context) {
	rn := &model.Runner{
		Name:    "built-in",
		URL:     s.cfg.RunnerURL,
		Tags:    []string{"default"},
		Builtin: true,
	}
	if err := s.store.UpsertRunner(ctx, rn); err != nil {
		s.log.Warn("seed built-in runner failed", "err", err)
	}
}

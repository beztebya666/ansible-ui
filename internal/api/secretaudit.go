package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/nikiv/ansible-ui/internal/model"
)

// recordSecretAccess persists a secret-access audit entry (best-effort). The
// actor is filled from the request context when blank, so HTTP-initiated
// accesses are attributed automatically; background callers (a run consuming a
// credential) pass the run's triggeredBy explicitly.
func (s *Server) recordSecretAccess(ctx context.Context, actor, action, credID, credName, detail string) {
	if actor == "" {
		actor = s.actor(ctx)
	}
	l := &model.SecretAccessLog{
		Actor: actor, Action: action, CredentialID: credID, CredentialName: credName, Detail: detail,
	}
	if err := s.store.LogSecretAccess(ctx, l); err != nil {
		s.log.Debug("secret-access record failed", "err", err)
	}
}

// handleListSecretAccessLogs returns the recent secret-access audit trail
// (admin-only — it concerns security-sensitive credential access).
func (s *Server) handleListSecretAccessLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	logs, err := s.store.ListSecretAccessLogs(r.Context(), limit)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	acts, err := s.store.ListActivity(r.Context(), limit)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acts)
}

// actor returns the authenticated username from the request context (set by
// authGuard), or "" for system contexts.
func (s *Server) actor(ctx context.Context) string {
	if u, ok := ctx.Value(userKey).(*model.User); ok && u != nil {
		return u.Username
	}
	return ""
}

// recordActivity persists an activity entry (best-effort) and publishes a live
// event so the feed updates without a refresh. A blank actor is filled from the
// request context, so every mutating handler is attributed automatically.
func (s *Server) recordActivity(ctx context.Context, actor, action, target, detail string) {
	if actor == "" {
		actor = s.actor(ctx)
	}
	a := &model.Activity{Actor: actor, Action: action, Target: target, Detail: detail}
	if err := s.store.AddActivity(ctx, a); err != nil {
		s.log.Debug("activity record failed", "err", err)
		return
	}
	s.events.Publish(map[string]any{"type": "activity", "activity": a})
}

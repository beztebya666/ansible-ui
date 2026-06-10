package api

import (
	"net/http"
	"strconv"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/notify"
)

// handleListNotificationLogs returns the recent notification delivery log
// (admin-only — it reveals who was alerted across every project).
func (s *Server) handleListNotificationLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	logs, err := s.store.ListNotificationLogs(r.Context(), limit)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	cs, err := s.store.ListNotificationChannels(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) handleCreateNotification(w http.ResponseWriter, r *http.Request) {
	var c model.NotificationChannel
	if err := decodeJSON(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if c.Type == "" {
		writeErr(w, http.StatusBadRequest, "type is required")
		return
	}
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	if c.ProjectID != nil && *c.ProjectID == "" {
		c.ProjectID = nil // empty = all projects
	}
	if err := s.store.CreateNotificationChannel(r.Context(), &c); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "notification.created", notifName(&c), c.Type)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleUpdateNotification(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetNotificationChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if err := decodeJSON(r, c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	c.ID = r.PathValue("id")
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	if c.ProjectID != nil && *c.ProjectID == "" {
		c.ProjectID = nil // empty = all projects
	}
	if err := s.store.UpdateNotificationChannel(r.Context(), c); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "notification.updated", notifName(c), c.Type)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteNotification(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if c, err := s.store.GetNotificationChannel(r.Context(), id); err == nil {
		name = notifName(c)
	}
	if err := s.store.DeleteNotificationChannel(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "notification.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// notifName is a channel's display name, falling back to its type.
func notifName(c *model.NotificationChannel) string {
	if c == nil {
		return ""
	}
	if c.Name != "" {
		return c.Name
	}
	return c.Type
}

// handleTestNotification sends a sample message to verify a channel's config.
func (s *Server) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetNotificationChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	testVars := map[string]string{"run": "test", "status": "test", "project": "ansible-ui"}
	if derr := notify.Dispatch(r.Context(), c, "ansible·ui test", "This is a test notification from ansible-ui.", "", testVars); derr != nil {
		writeErr(w, http.StatusBadGateway, "delivery failed: "+derr.Error())
		return
	}
	s.recordActivity(r.Context(), "", "notification.tested", notifName(c), c.Type)
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

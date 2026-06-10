package api

import (
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
)

// validCaps filters a permission list down to the known capabilities (dropping
// anything unrecognised) so a role can never grant a bogus permission.
func validCaps(perms []string) []string {
	known := map[string]bool{}
	for _, c := range model.AllCaps {
		known[c] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, p := range perms {
		if known[p] && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	return out
}

func (s *Server) handleListCustomRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.store.ListCustomRoles(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (s *Server) handleCreateCustomRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var c model.CustomRole
	if err := decodeJSON(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if c.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	c.Permissions = validCaps(c.Permissions)
	if err := s.store.CreateCustomRole(r.Context(), &c); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "role.created", c.Name, "")
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleUpdateCustomRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	c, err := s.store.GetCustomRole(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var in struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Permissions *[]string `json:"permissions"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		c.Name = *in.Name
	}
	if in.Description != nil {
		c.Description = *in.Description
	}
	if in.Permissions != nil {
		c.Permissions = validCaps(*in.Permissions)
	}
	if err := s.store.UpdateCustomRole(r.Context(), c); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "role.updated", c.Name, "")
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCustomRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.store.DeleteCustomRole(r.Context(), r.PathValue("id")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "role.deleted", r.PathValue("id"), "")
	w.WriteHeader(http.StatusNoContent)
}

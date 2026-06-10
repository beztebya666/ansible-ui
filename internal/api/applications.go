package api

import (
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := s.store.ListApplications(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (s *Server) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Icon     string   `json:"icon"`
		Bin      string   `json:"bin"`
		Args     []string `json:"args"`
		Priority int      `json:"priority"`
		Active   *bool    `json:"active"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name == "" || in.Bin == "" {
		writeErr(w, http.StatusBadRequest, "name and bin are required for a custom app")
		return
	}
	id := slugify(in.ID)
	if id == "" {
		id = slugify(in.Name)
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	a := &model.Application{
		ID: id, Name: in.Name, Icon: in.Icon, Bin: in.Bin, Args: in.Args,
		Priority: in.Priority, Active: active, Kind: model.AppKindCustom,
	}
	if err := s.store.UpsertApplication(r.Context(), a); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "application.created", a.Name, a.Bin)
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	a, err := s.store.GetApplication(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var in struct {
		Name     *string   `json:"name"`
		Icon     *string   `json:"icon"`
		Bin      *string   `json:"bin"`
		Args     *[]string `json:"args"`
		Priority *int      `json:"priority"`
		Active   *bool     `json:"active"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		a.Name = *in.Name
	}
	if in.Icon != nil {
		a.Icon = *in.Icon
	}
	// The binary is only editable for custom apps (built-ins use the runner dispatch).
	if in.Bin != nil && a.Kind == model.AppKindCustom {
		a.Bin = *in.Bin
	}
	if in.Args != nil {
		a.Args = *in.Args
	}
	if in.Priority != nil {
		a.Priority = *in.Priority
	}
	if in.Active != nil {
		a.Active = *in.Active
	}
	if err := s.store.UpsertApplication(r.Context(), a); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "application.updated", a.Name, a.Bin)
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	a, err := s.store.GetApplication(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if a.Kind == model.AppKindBuiltin {
		writeErr(w, http.StatusBadRequest, "built-in apps can't be deleted — disable it instead")
		return
	}
	if err := s.store.DeleteApplication(r.Context(), a.ID); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "application.deleted", a.Name, "")
	w.WriteHeader(http.StatusNoContent)
}

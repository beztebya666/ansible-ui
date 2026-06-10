package api

import (
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListViews(w http.ResponseWriter, r *http.Request) {
	vs, err := s.store.ListTemplateViews(r.Context(), projectScope(r))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (s *Server) handleCreateView(w http.ResponseWriter, r *http.Request) {
	var v model.TemplateView
	if err := decodeJSON(r, &v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if v.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if v.ProjectID == nil {
		v.ProjectID = scopePtr(projectScope(r))
	}
	if err := s.store.CreateTemplateView(r.Context(), &v); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handleUpdateView(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.GetTemplateView(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if err := decodeJSON(r, v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	v.ID = r.PathValue("id")
	if err := s.store.UpdateTemplateView(r.Context(), v); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteView(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTemplateView(r.Context(), r.PathValue("id")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

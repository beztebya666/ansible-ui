package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	toks, err := s.store.ListTokens(r.Context(), u.ID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toks)
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var in struct {
		Name        string `json:"name"`
		ExpiresDays int    `json:"expiresDays"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		writeErr(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	secret := "aui_" + hex.EncodeToString(raw)

	t := &model.APIToken{UserID: u.ID, Name: in.Name, Prefix: secret[:12]}
	if in.ExpiresDays > 0 {
		exp := time.Now().Add(time.Duration(in.ExpiresDays) * 24 * time.Hour)
		t.ExpiresAt = &exp
	}
	if err := s.store.CreateToken(r.Context(), t, hashToken(secret)); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	t.Token = secret // shown once
	s.recordActivity(r.Context(), u.Username, "token.created", t.Name, "")
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if err := s.store.DeleteToken(r.Context(), r.PathValue("id"), u.ID); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), u.Username, "token.revoked", "API token", "")
	w.WriteHeader(http.StatusNoContent)
}

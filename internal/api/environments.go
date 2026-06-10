package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := s.store.ListEnvironments(r.Context(), projectScope(r))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	for _, e := range envs {
		s.hydrateEnvForClient(e)
	}
	writeJSON(w, http.StatusOK, envs)
}

func (s *Server) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	e, err := s.store.GetEnvironment(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.hydrateEnvForClient(e)
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var e model.Environment
	if err := decodeJSON(r, &e); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if e.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if e.ExtraVars == nil {
		e.ExtraVars = map[string]any{}
	}
	if e.EnvVars == nil {
		e.EnvVars = map[string]string{}
	}
	if e.ProjectID == nil {
		e.ProjectID = scopePtr(projectScope(r))
	}
	blob, err := s.encodeEnvSecrets(cleanSecrets(e.Secrets))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "encrypt secrets: "+err.Error())
		return
	}
	e.SecretsBlob = blob
	if err := s.store.CreateEnvironment(r.Context(), &e); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "environment.created", e.Name, "")
	s.hydrateEnvForClient(&e)
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleUpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	existing, err := s.store.GetEnvironment(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	prior, _ := s.decodeEnvSecrets(existing.SecretsBlob)

	e := existing
	if err := decodeJSON(r, e); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	e.ID = r.PathValue("id")
	if e.ExtraVars == nil {
		e.ExtraVars = map[string]any{}
	}
	if e.EnvVars == nil {
		e.EnvVars = map[string]string{}
	}
	// Merge: an incoming secret with an empty value keeps its prior value (so the
	// UI never needs to re-enter secrets just to rename or edit something else).
	merged := mergeSecrets(prior, e.Secrets)
	blob, err := s.encodeEnvSecrets(merged)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "encrypt secrets: "+err.Error())
		return
	}
	e.SecretsBlob = blob
	if err := s.store.UpdateEnvironment(r.Context(), e); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.hydrateEnvForClient(e)
	s.recordActivity(r.Context(), "", "environment.updated", e.Name, "")
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if e, err := s.store.GetEnvironment(r.Context(), id); err == nil {
		name = e.Name
	}
	if err := s.store.DeleteEnvironment(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "environment.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- secret helpers ----------------------------------------------------

func (s *Server) encodeEnvSecrets(secrets []model.EnvSecret) ([]byte, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	plain, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	return s.cipher.Encrypt(plain)
}

func (s *Server) decodeEnvSecrets(blob []byte) ([]model.EnvSecret, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	plain, err := s.cipher.Decrypt(blob)
	if err != nil {
		return nil, err
	}
	var out []model.EnvSecret
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// hydrateEnvForClient replaces the (private) ciphertext with a values-free list
// of secret names/types so the client can render them without exposing secrets.
func (s *Server) hydrateEnvForClient(e *model.Environment) {
	secrets, _ := s.decodeEnvSecrets(e.SecretsBlob)
	out := make([]model.EnvSecret, 0, len(secrets))
	for _, sec := range secrets {
		out = append(out, model.EnvSecret{Name: sec.Name, Type: sec.Type, HasValue: sec.Value != ""})
	}
	e.Secrets = out
	e.SecretsBlob = nil
}

// envSecretsFor decrypts an environment's secrets for use at run launch.
func (s *Server) envSecretsFor(_ context.Context, e *model.Environment) []model.EnvSecret {
	secrets, _ := s.decodeEnvSecrets(e.SecretsBlob)
	return secrets
}

// cleanSecrets drops blank/nameless entries and normalises the type.
func cleanSecrets(in []model.EnvSecret) []model.EnvSecret {
	out := make([]model.EnvSecret, 0, len(in))
	for _, s := range in {
		if s.Name == "" || s.Value == "" {
			continue
		}
		if s.Type != model.EnvSecretVar {
			s.Type = model.EnvSecretEnv
		}
		out = append(out, model.EnvSecret{Name: s.Name, Type: s.Type, Value: s.Value})
	}
	return out
}

// mergeSecrets keeps prior values for incoming secrets whose value is blank, and
// drops any prior secret not present in the incoming list (deletion).
func mergeSecrets(prior, incoming []model.EnvSecret) []model.EnvSecret {
	priorByName := map[string]string{}
	for _, p := range prior {
		priorByName[p.Name] = p.Value
	}
	out := make([]model.EnvSecret, 0, len(incoming))
	for _, s := range incoming {
		if s.Name == "" {
			continue
		}
		val := s.Value
		if val == "" {
			val = priorByName[s.Name] // unchanged secret — reuse stored value
		}
		if val == "" {
			continue
		}
		typ := s.Type
		if typ != model.EnvSecretVar {
			typ = model.EnvSecretEnv
		}
		out = append(out, model.EnvSecret{Name: s.Name, Type: typ, Value: val})
	}
	return out
}

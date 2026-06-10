package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

func (s *Server) handleListRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.ListRepositories(r.Context(), projectScope(r))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (s *Server) handleGetRepository(w http.ResponseWriter, r *http.Request) {
	repo, err := s.store.GetRepository(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) handleCreateRepository(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string  `json:"name"`
		GitURL       string  `json:"gitUrl"`
		Branch       string  `json:"branch"`
		CredentialID *string `json:"credentialId"`
		CacheEnabled *bool   `json:"cacheEnabled"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name == "" || in.GitURL == "" {
		writeErr(w, http.StatusBadRequest, "name and gitUrl are required")
		return
	}
	repo := &model.Repository{Name: in.Name, GitURL: in.GitURL, Branch: in.Branch, CredentialID: emptyToNil(in.CredentialID), ProjectID: scopePtr(projectScope(r)), CacheEnabled: in.CacheEnabled == nil || *in.CacheEnabled}
	if err := s.store.CreateRepository(r.Context(), repo); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "repository.created", repo.Name, repo.GitURL)
	s.events.Publish(map[string]any{"type": "repository.updated", "repository": repo})
	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) handleUpdateRepository(w http.ResponseWriter, r *http.Request) {
	repo, err := s.store.GetRepository(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var in struct {
		Name         *string `json:"name"`
		GitURL       *string `json:"gitUrl"`
		Branch       *string `json:"branch"`
		CredentialID *string `json:"credentialId"`
		CacheEnabled *bool   `json:"cacheEnabled"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		repo.Name = *in.Name
	}
	if in.GitURL != nil {
		repo.GitURL = *in.GitURL
	}
	if in.Branch != nil {
		repo.Branch = *in.Branch
	}
	if in.CredentialID != nil {
		repo.CredentialID = emptyToNil(in.CredentialID)
	}
	if in.CacheEnabled != nil {
		repo.CacheEnabled = *in.CacheEnabled
	}
	if err := s.store.UpdateRepository(r.Context(), repo); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "repository.updated", repo.Name, repo.GitURL)
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) handleDeleteRepository(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if repo, err := s.store.GetRepository(r.Context(), id); err == nil {
		name = repo.Name
	}
	if err := s.store.DeleteRepository(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.cfg.DataDir, "repos", id))
	s.recordActivity(r.Context(), "", "repository.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// handleSyncRepository clones/pulls the repo via the runner and records the result.
func (s *Server) handleSyncRepository(w http.ResponseWriter, r *http.Request) {
	repo, err := s.store.GetRepository(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	res, serr := s.syncRepository(r.Context(), repo)
	if serr != nil {
		writeErr(w, http.StatusBadGateway, serr.Error())
		return
	}
	if res.Error != "" {
		writeJSON(w, http.StatusOK, map[string]any{"repository": repo, "error": res.Error})
		return
	}
	s.recordActivity(r.Context(), "", "repository.synced", repo.Name, res.Commit)
	writeJSON(w, http.StatusOK, map[string]any{"repository": repo, "commit": res.Commit, "message": res.Message})
}

// repoSyncSpec builds the git-sync request for a repository, attaching SSH key /
// HTTP credentials from the linked Key Store entry when present.
func (s *Server) repoSyncSpec(ctx context.Context, repo *model.Repository) model.GitSyncSpec {
	spec := model.GitSyncSpec{RepoID: repo.ID, GitURL: repo.GitURL, Branch: repo.Branch}
	if repo.CredentialID != nil && *repo.CredentialID != "" {
		if sec, err := s.credentialSecret(ctx, *repo.CredentialID); err == nil && sec != nil {
			spec.SSHPrivateKey = sec.SSHPrivateKey
			spec.Password = sec.Password
		}
		if cred, err := s.store.GetCredential(ctx, *repo.CredentialID); err == nil {
			spec.Username = cred.Login
			spec.SSHCertificate = cred.SSHCertificate
		}
	}
	return spec
}

// syncRepository performs a sync and updates persisted status; returns the
// runner result (whose .Error is non-empty on a git failure).
func (s *Server) syncRepository(ctx context.Context, repo *model.Repository) (*model.GitSyncResult, error) {
	_ = s.store.SetRepoStatus(ctx, repo.ID, model.RepoSyncing, "", "", false)
	repo.Status = model.RepoSyncing
	s.events.Publish(map[string]any{"type": "repository.updated", "repository": repo})

	spec := s.repoSyncSpec(ctx, repo)

	res, err := runnerclient.SyncGit(ctx, s.cfg.RunnerHTTP, spec)
	if err != nil {
		_ = s.store.SetRepoStatus(ctx, repo.ID, model.RepoError, "", err.Error(), false)
		repo.Status = model.RepoError
		repo.LastError = err.Error()
		s.events.Publish(map[string]any{"type": "repository.updated", "repository": repo})
		return nil, err
	}
	if res.Error != "" {
		_ = s.store.SetRepoStatus(ctx, repo.ID, model.RepoError, "", res.Error, false)
		repo.Status = model.RepoError
		repo.LastError = res.Error
	} else {
		_ = s.store.SetRepoStatus(ctx, repo.ID, model.RepoReady, res.Commit, "", true)
		repo.Status = model.RepoReady
		repo.LastCommit = res.Commit
		repo.LastError = ""
	}
	s.events.Publish(map[string]any{"type": "repository.updated", "repository": repo})
	return res, nil
}

// handleRepoTree returns the file tree of a synced repository (for picking a
// project subfolder).
func (s *Server) handleRepoTree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dir := filepath.Join(s.cfg.DataDir, "repos", id)
	if _, err := os.Stat(dir); err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, buildTree(dir, dir, 0))
}

func emptyToNil(p *string) *string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil
	}
	return p
}

package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// artifactDir is where a run's captured files live on disk.
func (s *Server) artifactDir(runID string) string {
	return filepath.Join(s.cfg.DataDir, "artifacts", runID)
}

// saveArtifact persists one file a runner streamed back (FrameArtifact) and
// indexes it. Captured on the runner, so it works for any runner (built-in or
// remote, shared /data or not). Path-escape guarded; best-effort (never fatal).
func (s *Server) saveArtifact(run *model.Run, name string, content []byte) {
	clean := filepath.ToSlash(strings.TrimPrefix(name, "/"))
	if clean == "" || strings.Contains(clean, "..") {
		s.log.Warn("rejected artifact name", "run", run.ID, "name", name)
		return
	}
	base := s.artifactDir(run.ID)
	dst := filepath.Join(base, filepath.FromSlash(clean))
	if rel, err := filepath.Rel(base, dst); err != nil || strings.HasPrefix(rel, "..") {
		s.log.Warn("rejected artifact path", "run", run.ID, "name", name)
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		s.log.Warn("artifact mkdir failed", "run", run.ID, "err", err)
		return
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		s.log.Warn("artifact write failed", "run", run.ID, "file", clean, "err", err)
		return
	}
	if err := s.store.CreateRunArtifact(context.Background(), &model.RunArtifact{RunID: run.ID, Name: clean, Size: int64(len(content))}); err != nil {
		s.log.Warn("artifact index failed", "run", run.ID, "file", clean, "err", err)
		return
	}
	s.log.Info("saved artifact", "run", run.ID, "file", clean, "bytes", len(content))
}

// handleListArtifacts returns a run's captured artifacts.
func (s *Server) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	arts, err := s.store.ListRunArtifacts(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if arts == nil {
		arts = []*model.RunArtifact{}
	}
	writeJSON(w, http.StatusOK, arts)
}

// handleDownloadArtifact streams one artifact as an attachment.
func (s *Server) handleDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	art, err := s.store.GetRunArtifact(r.Context(), r.PathValue("artifactId"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if art.RunID != r.PathValue("id") {
		writeErr(w, http.StatusNotFound, "artifact not found for this run")
		return
	}
	base := s.artifactDir(art.RunID)
	full := filepath.Join(base, filepath.FromSlash(art.Name))
	// Guard against path traversal: the resolved file must stay under the run's dir.
	if rel, err := filepath.Rel(base, full); err != nil || strings.HasPrefix(rel, "..") {
		writeErr(w, http.StatusBadRequest, "invalid artifact path")
		return
	}
	f, err := os.Open(full)
	if err != nil {
		writeErr(w, http.StatusNotFound, "artifact file missing on disk")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(art.Name)+`"`)
	_, _ = io.Copy(w, f)
}

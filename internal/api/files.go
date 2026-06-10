package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

// fileNode is one entry in the project file tree.
type fileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // relative to project root, slash-separated
	Type     string      `json:"type"` // "dir" | "file"
	Size     int64       `json:"size,omitempty"`
	Children []*fileNode `json:"children,omitempty"`
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	root := s.projectDir(p)
	tree := buildTree(root, root, 0)
	writeJSON(w, http.StatusOK, tree)
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	abs, err := safeJoin(s.projectDir(p), r.URL.Query().Get("path"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	if info.Size() > 2<<20 {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large to edit")
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":    r.URL.Query().Get("path"),
		"content": string(data),
		"size":    info.Size(),
	})
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, p.ID, model.CapEdit) {
		return
	}
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	abs, err := safeJoin(s.projectDir(p), in.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), "", "file.saved", in.Path, p.Name)
	writeJSON(w, http.StatusOK, map[string]any{"path": in.Path, "saved": true})
}

// handleDeleteFile removes a file or directory (recursively) from the project.
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, p.ID, model.CapEdit) {
		return
	}
	root := s.projectDir(p)
	abs, err := safeJoin(root, r.URL.Query().Get("path"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if abs == root {
		writeErr(w, http.StatusBadRequest, "cannot delete the project root")
		return
	}
	if err := os.RemoveAll(abs); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "file.deleted", r.URL.Query().Get("path"), p.Name)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handleRenameFile moves/renames a file or directory within the project.
func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, p.ID, model.CapEdit) {
		return
	}
	var in struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	root := s.projectDir(p)
	from, err := safeJoin(root, in.From)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	to, err := safeJoin(root, in.To)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if from == root || to == root {
		writeErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	if _, err := os.Stat(to); err == nil {
		writeErr(w, http.StatusConflict, "a file with that name already exists")
		return
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.Rename(from, to); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "file.renamed", in.From+" → "+in.To, p.Name)
	writeJSON(w, http.StatusOK, map[string]any{"path": in.To, "renamed": true})
}

// handleGitCommit commits an edited file from a git-backed project onto a NEW
// branch and pushes it — never onto the (often protected) base branch. The
// operator opens a PR/MR from the returned branch.
func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, p.ID, model.CapEdit) {
		return
	}
	if p.RepositoryID == nil || *p.RepositoryID == "" {
		writeErr(w, http.StatusBadRequest, "project is not git-backed — edit files directly")
		return
	}
	var in struct {
		Path    string `json:"path"`    // single-file (legacy)
		Content string `json:"content"` //
		Files   []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"` // multi-file staged commit
		Message string `json:"message"`
		Branch  string `json:"branch"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if len(in.Files) == 0 && in.Path != "" {
		in.Files = append(in.Files, struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}{in.Path, in.Content})
	}
	if len(in.Files) == 0 {
		writeErr(w, http.StatusBadRequest, "path or files are required")
		return
	}
	repo, err := s.store.GetRepository(r.Context(), *p.RepositoryID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	branch := sanitizeBranch(in.Branch)
	if branch == "" {
		branch = "aui/edit-" + time.Now().Format("20060102-150405")
	}
	if branch == repo.Branch {
		writeErr(w, http.StatusBadRequest, "choose a new branch — pushing onto the base branch is not allowed")
		return
	}
	// Build the commit's files (repo-relative = sub_path + editor path) and keep
	// the editor-relative paths for the activity + branch record.
	var editorPaths []string
	gitFiles := make([]model.GitFile, 0, len(in.Files))
	for _, f := range in.Files {
		ep := filepath.ToSlash(strings.TrimPrefix(f.Path, "/"))
		editorPaths = append(editorPaths, ep)
		gitFiles = append(gitFiles, model.GitFile{Path: path.Join(p.SubPath, ep), Content: f.Content})
	}
	msg := strings.TrimSpace(in.Message)
	if msg == "" {
		if len(editorPaths) == 1 {
			msg = "Update " + path.Base(editorPaths[0]) + " via ansible-ui"
		} else {
			msg = "Update " + strconv.Itoa(len(editorPaths)) + " files via ansible-ui"
		}
	}

	spec := s.repoCommitSpec(r.Context(), repo)
	spec.NewBranch = branch
	spec.Message = msg
	spec.Files = gitFiles
	if u := currentUser(r); u != nil {
		spec.AuthorName = u.Username
		spec.AuthorEmail = u.Email
	}

	res, cerr := runnerclient.CommitGit(r.Context(), s.cfg.RunnerHTTP, spec)
	if cerr != nil {
		writeErr(w, http.StatusBadGateway, "commit failed: "+cerr.Error())
		return
	}
	if res.Error != "" {
		writeErr(w, http.StatusBadRequest, "commit failed: "+res.Error)
		return
	}
	prURL := prCompareURL(repo.GitURL, repo.Branch, branch)
	_ = s.store.RecordPushedBranch(r.Context(), &model.PushedBranch{
		ProjectID: p.ID, Branch: res.Branch, Commit: res.Commit, PRURL: prURL,
		Files: editorPaths, Actor: s.actor(r.Context()),
	})
	s.recordActivity(r.Context(), s.actor(r.Context()), "file.committed", strings.Join(editorPaths, ", ")+" → "+branch, p.Name)
	s.events.Publish(map[string]any{"type": "branches", "projectId": p.ID})
	writeJSON(w, http.StatusOK, map[string]any{
		"branch": res.Branch,
		"commit": res.Commit,
		"pushed": res.Pushed,
		"prUrl":  prURL,
	})
}

// handleListBranches returns the branches the UI pushed for a git-backed project.
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := s.store.ListPushedBranches(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if branches == nil {
		branches = []*model.PushedBranch{}
	}
	writeJSON(w, http.StatusOK, branches)
}

// repoCommitSpec builds the credentialed commit request for a repository.
func (s *Server) repoCommitSpec(ctx context.Context, repo *model.Repository) model.GitCommitSpec {
	spec := model.GitCommitSpec{RepoID: repo.ID, GitURL: repo.GitURL, BaseBranch: repo.Branch}
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

// sanitizeBranch keeps a git-safe branch name (no spaces / control chars).
func sanitizeBranch(b string) string {
	b = strings.TrimSpace(b)
	b = strings.Map(func(r rune) rune {
		if r <= ' ' || r == '~' || r == '^' || r == ':' || r == '?' || r == '*' || r == '[' || r == '\\' {
			return '-'
		}
		return r
	}, b)
	return strings.Trim(b, "/-")
}

// prCompareURL builds a "compare/open a PR" link for GitHub/GitLab https remotes.
func prCompareURL(gitURL, base, branch string) string {
	u := strings.TrimSuffix(gitURL, ".git")
	// normalise scp-style git@host:owner/repo
	if strings.HasPrefix(u, "git@") {
		u = strings.Replace(u, ":", "/", 1)
		u = "https://" + strings.TrimPrefix(u, "git@")
	}
	switch {
	case strings.Contains(u, "github.com"):
		return u + "/compare/" + base + "..." + branch + "?expand=1"
	case strings.Contains(u, "gitlab"):
		return u + "/-/merge_requests/new?merge_request%5Bsource_branch%5D=" + branch
	}
	return ""
}

// buildTree returns the sorted (dirs first) file tree under dir.
func buildTree(root, dir string, depth int) []*fileNode {
	if depth > 12 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var nodes []*fileNode
	for _, e := range entries {
		name := e.Name()
		if name == ".git" || name == "node_modules" || name == "__pycache__" {
			continue
		}
		full := filepath.Join(dir, name)
		rel, _ := filepath.Rel(root, full)
		n := &fileNode{Name: name, Path: filepath.ToSlash(rel)}
		if e.IsDir() {
			n.Type = "dir"
			n.Children = buildTree(root, full, depth+1)
		} else {
			n.Type = "file"
			if info, ierr := e.Info(); ierr == nil {
				n.Size = info.Size()
			}
		}
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "dir"
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// safeJoin joins rel onto base, refusing paths that escape base.
func safeJoin(base, rel string) (string, error) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if rel == "" {
		return "", errors.New("path is required")
	}
	clean := filepath.Join(base, filepath.FromSlash(rel))
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absClean, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	if absClean != absBase && !strings.HasPrefix(absClean, absBase+string(os.PathSeparator)) {
		return "", errors.New("path escapes project root")
	}
	return absClean, nil
}

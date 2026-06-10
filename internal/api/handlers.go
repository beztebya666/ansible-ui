package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

// ---- projects ----------------------------------------------------------

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	p.Playbooks = s.listPlaybooks(s.projectDir(p))
	p.InventoryFiles = s.listInventoryFiles(s.projectDir(p))
	p.MyRole = s.effectiveProjectRole(r, p.ID, currentUser(r))
	if p.RepositoryID != nil {
		if repo, rerr := s.store.GetRepository(r.Context(), *p.RepositoryID); rerr == nil {
			p.RepositoryName = repo.Name
		}
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	// Project creation is admin-only unless the "non-admin can create project"
	// feature flag is enabled.
	if u := currentUser(r); u == nil || (u.Role != "admin" && !s.boolSetting(r.Context(), settingNonadminCreateProject)) {
		writeErr(w, http.StatusForbidden, "only admins can create projects")
		return
	}
	var in struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Description  string `json:"description"`
		RepositoryID string `json:"repositoryId"`
		SubPath      string `json:"subPath"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	slug := slugify(in.Slug)
	if slug == "" {
		slug = slugify(in.Name)
	}
	p := &model.Project{Name: in.Name, Slug: slug, Description: in.Description}

	if in.RepositoryID != "" {
		// Repo-backed: files come from the git checkout — no local dir.
		if _, err := s.store.GetRepository(r.Context(), in.RepositoryID); err != nil {
			writeErr(w, http.StatusBadRequest, "repository not found")
			return
		}
		repoID := in.RepositoryID
		p.SourceType = "git"
		p.RepositoryID = &repoID
		p.SubPath = strings.Trim(filepath.ToSlash(in.SubPath), "/")
	} else {
		p.SourceType = "local"
		p.Path = filepath.ToSlash(filepath.Join("projects", slug))
		if err := os.MkdirAll(s.projectDir(p), 0o755); err != nil {
			writeErr(w, http.StatusInternalServerError, "create dir: "+err.Error())
			return
		}
	}

	if err := s.store.CreateProject(r.Context(), p); err != nil {
		writeErr(w, http.StatusConflict, "create project (slug taken?): "+err.Error())
		return
	}
	s.recordActivity(r.Context(), "", "project.created", p.Name, "")
	s.events.Publish(map[string]any{"type": "project.created", "project": p})
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, id, model.CapManage) {
		return
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Slug        *string `json:"slug"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.Slug != nil {
		if sl := slugify(*in.Slug); sl != "" {
			p.Slug = sl
		}
	}
	if p.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.store.UpdateProject(r.Context(), id, p.Name, p.Description, p.Slug); err != nil {
		writeErr(w, http.StatusConflict, "could not save (tag already in use?): "+err.Error())
		return
	}
	s.recordActivity(r.Context(), "", "project.updated", p.Name, "")
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, id, model.CapManage) {
		return
	}
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	// Remove the working directory if it lives under our data root.
	dir := s.projectDir(p)
	if rel, rerr := filepath.Rel(s.cfg.DataDir, dir); rerr == nil && !strings.HasPrefix(rel, "..") {
		_ = os.RemoveAll(dir)
	}
	s.recordActivity(r.Context(), "", "project.deleted", p.Name, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListPlaybooks(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.listPlaybooks(s.projectDir(p)))
}

// ---- inventories -------------------------------------------------------

func (s *Server) handleListInventories(w http.ResponseWriter, r *http.Request) {
	invs, err := s.store.ListInventories(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invs)
}

func (s *Server) handleCreateInventory(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name              string   `json:"name"`
		Type              string   `json:"type"`
		Content           string   `json:"content"`
		Provider          string   `json:"provider"`
		CredentialID      *string  `json:"credentialId"`
		Region            string   `json:"region"`
		RunnerTag         string   `json:"runnerTag"`
		ConnCredentialIDs []string `json:"connCredentialIds"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if !s.requireProjectCap(w, r, r.PathValue("id"), model.CapEdit) {
		return
	}
	inv := &model.Inventory{
		ProjectID: r.PathValue("id"), Name: in.Name, Type: in.Type, Content: in.Content,
		Provider: in.Provider, CredentialID: in.CredentialID, Region: in.Region, RunnerTag: strings.TrimSpace(in.RunnerTag),
		ConnCredentialIDs: in.ConnCredentialIDs,
	}
	if err := s.store.CreateInventory(r.Context(), inv); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "inventory.created", inv.Name, inv.Type)
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) handleGetInventory(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func (s *Server) handleUpdateInventory(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, inv.ProjectID, model.CapEdit) {
		return
	}
	var in struct {
		Name              *string   `json:"name"`
		Type              *string   `json:"type"`
		Content           *string   `json:"content"`
		Provider          *string   `json:"provider"`
		CredentialID      *string   `json:"credentialId"`
		Region            *string   `json:"region"`
		RunnerTag         *string   `json:"runnerTag"`
		ConnCredentialIDs *[]string `json:"connCredentialIds"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		inv.Name = *in.Name
	}
	if in.Type != nil {
		inv.Type = *in.Type
	}
	if in.Content != nil {
		inv.Content = *in.Content
	}
	if in.Provider != nil {
		inv.Provider = *in.Provider
	}
	if in.CredentialID != nil {
		inv.CredentialID = in.CredentialID
	}
	if in.Region != nil {
		inv.Region = *in.Region
	}
	if in.RunnerTag != nil {
		inv.RunnerTag = strings.TrimSpace(*in.RunnerTag)
	}
	if in.ConnCredentialIDs != nil {
		inv.ConnCredentialIDs = *in.ConnCredentialIDs
	}
	if err := s.store.UpdateInventory(r.Context(), inv); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "inventory.updated", inv.Name, inv.Type)
	writeJSON(w, http.StatusOK, inv)
}

func (s *Server) handleDeleteInventory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if inv, err := s.store.GetInventory(r.Context(), id); err == nil {
		if !s.requireProjectCap(w, r, inv.ProjectID, model.CapEdit) {
			return
		}
		name = inv.Name
	}
	if err := s.store.DeleteInventory(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "inventory.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- templates ---------------------------------------------------------

func (s *Server) handleProjectTemplates(w http.ResponseWriter, r *http.Request) {
	ts, err := s.store.ListTemplates(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ts)
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("projectId")
	if pid == "" {
		pid = projectScope(r)
	}
	ts, err := s.store.ListTemplates(r.Context(), pid)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	// Hydrate the Semaphore-style VERSION: each build template shows its own
	// latest successful build; a deploy shows the build it would roll out.
	if versions, verr := s.store.LatestBuildVersions(r.Context(), pid); verr == nil {
		for _, t := range ts {
			switch t.Type {
			case model.TemplateTypeBuild:
				t.LastBuildVersion = versions[t.ID]
			case model.TemplateTypeDeploy:
				if t.BuildTemplateID != nil {
					t.LastBuildVersion = versions[*t.BuildTemplateID]
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, ts)
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var t model.Template
	if err := decodeJSON(r, &t); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if t.App == "" {
		t.App = model.AppAnsible
	}
	if t.Type != model.TemplateTypeBuild && t.Type != model.TemplateTypeDeploy {
		t.Type = model.TemplateTypeTask
	}
	if t.Type != model.TemplateTypeDeploy {
		t.BuildTemplateID = nil
	}
	// Infra apps (terraform/tofu/terragrunt) run a working directory, not a file;
	// an empty entrypoint means the project root.
	if (model.IsInfraApp(t.App) || t.App == model.AppPulumi) && t.Playbook == "" {
		t.Playbook = "."
	}
	if t.ProjectID == "" || t.Name == "" || t.Playbook == "" {
		writeErr(w, http.StatusBadRequest, "projectId, name and playbook/entrypoint are required")
		return
	}
	if !s.requireProjectCap(w, r, t.ProjectID, model.CapEdit) {
		return
	}
	if t.ExtraVars == nil {
		t.ExtraVars = map[string]any{}
	}
	if err := s.store.CreateTemplate(r.Context(), &t); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "template.created", t.Name, t.App)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, t.ProjectID, model.CapEdit) {
		return
	}
	if err := decodeJSON(r, t); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	t.ID = r.PathValue("id")
	if t.ExtraVars == nil {
		t.ExtraVars = map[string]any{}
	}
	if t.Type != model.TemplateTypeBuild && t.Type != model.TemplateTypeDeploy {
		t.Type = model.TemplateTypeTask
	}
	if t.Type != model.TemplateTypeDeploy {
		t.BuildTemplateID = nil
	}
	if err := s.store.UpdateTemplate(r.Context(), t); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "template.updated", t.Name, t.App)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if t, err := s.store.GetTemplate(r.Context(), name); err == nil {
		if !s.requireProjectCap(w, r, t.ProjectID, model.CapEdit) {
			return
		}
		name = t.Name
	}
	if err := s.store.DeleteTemplate(r.Context(), r.PathValue("id")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "template.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- stats -------------------------------------------------------------

// handleInsights returns aggregated run analytics over a recent window.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	ins, err := s.store.RunInsights(r.Context(), r.URL.Query().Get("projectId"), days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ins)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, _ := s.store.ListProjects(ctx)
	templates, _ := s.store.ListTemplates(ctx, "")
	byStatus, _ := s.store.CountRunsByStatus(ctx)
	recent, _ := s.store.ListRuns(ctx, store.RunFilter{Limit: 8})

	total := 0
	for _, n := range byStatus {
		total += n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects":    len(projects),
		"templates":   len(templates),
		"runsTotal":   total,
		"runsByState": byStatus,
		"active":      byStatus[model.StatusRunning] + byStatus[model.StatusPending] + byStatus[model.StatusQueued],
		"recentRuns":  recent,
	})
}

// ---- helpers -----------------------------------------------------------

// projectDir resolves a project's working directory. Repo-backed projects live
// under the repository checkout at <repos>/<repoId>/<subPath>.
func (s *Server) projectDir(p *model.Project) string {
	if p.RepositoryID != nil && *p.RepositoryID != "" {
		return filepath.Join(s.cfg.DataDir, "repos", *p.RepositoryID, filepath.FromSlash(p.SubPath))
	}
	return filepath.Join(s.cfg.DataDir, filepath.FromSlash(p.Path))
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

var playbookHint = regexp.MustCompile(`(?m)^\s*-?\s*hosts:\s`)

// listInventoryFiles walks a project and returns the relative paths of files
// that look like Ansible inventories: *.ini, files named hosts/inventory*, or
// anything under an inventory/ or inventories/ directory. So inventories show up
// automatically (no manual entry) and can be picked from the file tree.
func (s *Server) listInventoryFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "roles" || name == "group_vars" || name == "host_vars" ||
				name == "collections" || strings.HasPrefix(name, ".") {
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := strings.ToLower(d.Name())
		ext := strings.ToLower(filepath.Ext(base))
		lower := "/" + strings.ToLower(rel) + "/"
		inInvDir := strings.Contains(lower, "/inventory/") || strings.Contains(lower, "/inventories/")
		looks := ext == ".ini" ||
			base == "hosts" || base == "inventory" ||
			strings.HasPrefix(base, "inventory.") || strings.HasPrefix(base, "hosts.") ||
			inInvDir
		// Exclude obvious non-inventory configs.
		if base == "ansible.cfg" || base == "tox.ini" || base == "setup.cfg" {
			looks = false
		}
		if looks {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// listPlaybooks walks a project and returns the relative paths of files that
// look like playbooks (a YAML list with a top-level hosts: key).
func (s *Server) listPlaybooks(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "roles" || name == "group_vars" || name == "host_vars" ||
				name == "collections" || strings.HasPrefix(name, ".") {
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil || !playbookHint.Match(data) {
			return nil
		}
		if rel, rerr := filepath.Rel(dir, path); rerr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// Project export/import — a portable JSON bundle of a project's structure
// (inventories, templates, workflows, recurring schedules) for backup or
// migration between instances. Cross-references are by NAME so the importer can
// remap them to fresh ids. Secrets are NOT exported: credential / environment /
// vault / repository links are dropped on import and must be re-linked.

type projectBundle struct {
	Version     int               `json:"version"`
	ExportedAt  time.Time         `json:"exportedAt"`
	Project     bundleProject     `json:"project"`
	Inventories []bundleInventory `json:"inventories"`
	Templates   []bundleTemplate  `json:"templates"`
	Workflows   []bundleWorkflow  `json:"workflows"`
	Schedules   []bundleSchedule  `json:"schedules"`
}

type bundleProject struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SourceType  string `json:"sourceType"`
	SubPath     string `json:"subPath,omitempty"`
}

type bundleInventory struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	Provider  string `json:"provider,omitempty"`
	Region    string `json:"region,omitempty"`
	RunnerTag string `json:"runnerTag,omitempty"`
}

type bundleTemplate struct {
	model.Template
	InventoryName     string `json:"inventoryName,omitempty"`
	BuildTemplateName string `json:"buildTemplateName,omitempty"`
}

type bundleWorkflow struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []bundleWFStep `json:"steps"`
}

type bundleWFStep struct {
	Name         string `json:"name"`
	TemplateName string `json:"templateName"`
	Condition    string `json:"condition"`
}

type bundleSchedule struct {
	Name         string `json:"name"`
	Cron         string `json:"cron"`
	TemplateName string `json:"templateName"`
	Active       bool   `json:"active"`
}

// handleExportProject serialises a project's structure to a JSON bundle.
func (s *Server) handleExportProject(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	proj, err := s.store.GetProject(r.Context(), pid)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, pid, model.CapManage) {
		return
	}
	ctx := r.Context()
	invs, _ := s.store.ListInventories(ctx, pid)
	tpls, _ := s.store.ListTemplates(ctx, pid)
	wfs, _ := s.store.ListWorkflows(ctx, pid)
	invName := map[string]string{}
	for _, inv := range invs {
		invName[inv.ID] = inv.Name
	}
	tplName := map[string]string{}
	for _, t := range tpls {
		tplName[t.ID] = t.Name
	}

	b := projectBundle{
		Version: 1, ExportedAt: time.Now(),
		Project: bundleProject{Name: proj.Name, Description: proj.Description, SourceType: proj.SourceType, SubPath: proj.SubPath},
	}
	for _, inv := range invs {
		b.Inventories = append(b.Inventories, bundleInventory{
			Name: inv.Name, Type: inv.Type, Content: inv.Content, Provider: inv.Provider, Region: inv.Region, RunnerTag: inv.RunnerTag,
		})
	}
	for _, t := range tpls {
		bt := bundleTemplate{Template: *t}
		if t.InventoryID != nil {
			bt.InventoryName = invName[*t.InventoryID]
		}
		if t.BuildTemplateID != nil {
			bt.BuildTemplateName = tplName[*t.BuildTemplateID]
		}
		// Strip instance-specific ids/links + secret references.
		bt.ID, bt.ProjectID = "", ""
		bt.InventoryID, bt.EnvironmentID, bt.VaultCredentialID, bt.BuildTemplateID = nil, nil, nil, nil
		bt.Vaults = nil
		bt.CreatedAt, bt.UpdatedAt, bt.LastBuildVersion = time.Time{}, time.Time{}, ""
		b.Templates = append(b.Templates, bt)
	}
	for _, wf := range wfs {
		bw := bundleWorkflow{Name: wf.Name, Description: wf.Description}
		for _, st := range wf.Steps {
			bw.Steps = append(bw.Steps, bundleWFStep{Name: st.Name, TemplateName: tplName[st.TemplateID], Condition: st.Condition})
		}
		b.Workflows = append(b.Workflows, bw)
	}
	for _, t := range tpls {
		scs, _ := s.store.ListSchedules(ctx, t.ID)
		for _, sc := range scs {
			if sc.Cron == "" {
				continue // skip one-time schedules — not portable
			}
			b.Schedules = append(b.Schedules, bundleSchedule{Name: sc.Name, Cron: sc.Cron, TemplateName: t.Name, Active: sc.Active})
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="project-`+proj.Slug+`.json"`)
	writeJSON(w, http.StatusOK, b)
}

// handleImportProject recreates a project + its structure from a bundle (admin).
func (s *Server) handleImportProject(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var b projectBundle
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid bundle: "+err.Error())
		return
	}
	if b.Project.Name == "" {
		writeErr(w, http.StatusBadRequest, "bundle has no project name")
		return
	}
	ctx := r.Context()
	name := b.Project.Name + " (imported)"
	slug := slugify(name)
	p := &model.Project{Name: name, Slug: slug, Description: b.Project.Description}
	// Repositories aren't exported, so import as a local project (re-link a repo
	// afterwards for a git-backed one).
	p.SourceType = "local"
	p.Path = "projects/" + slug
	if err := s.store.CreateProject(ctx, p); err != nil {
		writeErr(w, http.StatusConflict, "create project (name may already exist): "+err.Error())
		return
	}

	invMap := map[string]string{} // name → new id
	for _, bi := range b.Inventories {
		inv := &model.Inventory{ProjectID: p.ID, Name: bi.Name, Type: bi.Type, Content: bi.Content, Provider: bi.Provider, Region: bi.Region, RunnerTag: bi.RunnerTag}
		if err := s.store.CreateInventory(ctx, inv); err == nil {
			invMap[bi.Name] = inv.ID
		}
	}
	tplMap := map[string]string{} // name → new id
	for i := range b.Templates {
		t := b.Templates[i].Template // value copy
		t.ID, t.ProjectID = "", p.ID
		t.InventoryID, t.EnvironmentID, t.VaultCredentialID, t.BuildTemplateID = nil, nil, nil, nil
		t.Vaults = nil
		if n := b.Templates[i].InventoryName; n != "" {
			if id, ok := invMap[n]; ok {
				t.InventoryID = &id
			}
		}
		if err := s.store.CreateTemplate(ctx, &t); err == nil {
			tplMap[t.Name] = t.ID
		}
	}
	// Second pass: link deploy → build templates now that all ids exist.
	for i := range b.Templates {
		bt := b.Templates[i]
		if bt.BuildTemplateName == "" {
			continue
		}
		myID, ok1 := tplMap[bt.Template.Name]
		buildID, ok2 := tplMap[bt.BuildTemplateName]
		if ok1 && ok2 {
			if t, err := s.store.GetTemplate(ctx, myID); err == nil {
				t.BuildTemplateID = &buildID
				_ = s.store.UpdateTemplate(ctx, t)
			}
		}
	}
	for _, bw := range b.Workflows {
		wf := &model.Workflow{ProjectID: p.ID, Name: bw.Name, Description: bw.Description}
		for _, st := range bw.Steps {
			wf.Steps = append(wf.Steps, model.WorkflowStep{Name: st.Name, TemplateID: tplMap[st.TemplateName], Condition: st.Condition})
		}
		_ = s.store.CreateWorkflow(ctx, wf)
	}
	for _, bs := range b.Schedules {
		tid, ok := tplMap[bs.TemplateName]
		if !ok || bs.Cron == "" {
			continue
		}
		sched, err := parseCron(bs.Cron)
		if err != nil {
			continue
		}
		next := sched.Next(time.Now())
		_ = s.store.CreateSchedule(ctx, &model.Schedule{TemplateID: tid, Name: bs.Name, Cron: bs.Cron, Active: bs.Active, NextRunAt: &next})
	}
	s.recordActivity(ctx, s.actor(ctx), "project.imported", p.Name,
		strconv.Itoa(len(b.Templates))+" templates")
	s.events.Publish(map[string]any{"type": "project.created"})
	writeJSON(w, http.StatusCreated, p)
}

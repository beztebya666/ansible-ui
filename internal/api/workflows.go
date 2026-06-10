package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// evalWhen evaluates a small step guard against the workflow variables. Supported:
//
//	"key"             → key is present + truthy (non-empty, not false/0/no/off)
//	"!key"            → key is absent or falsy
//	"a == b" / "a != b" → string compare; each side is a quoted literal, a variable
//	                      name (resolved from vars), or a bare literal word.
func evalWhen(expr string, vars map[string]any) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}
	if i := strings.Index(expr, "=="); i >= 0 {
		return resolveWhenToken(expr[:i], vars) == resolveWhenToken(expr[i+2:], vars)
	}
	if i := strings.Index(expr, "!="); i >= 0 {
		return resolveWhenToken(expr[:i], vars) != resolveWhenToken(expr[i+2:], vars)
	}
	if strings.HasPrefix(expr, "!") {
		return !varTruthy(expr[1:], vars)
	}
	return varTruthy(expr, vars)
}

func resolveWhenToken(s string, vars map[string]any) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1] // quoted literal
	}
	if v, ok := vars[s]; ok {
		return fmt.Sprintf("%v", v)
	}
	return s // bare literal
}

func varTruthy(key string, vars map[string]any) bool {
	v, ok := vars[strings.TrimSpace(key)]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v))) {
	case "", "false", "0", "no", "off":
		return false
	}
	return true
}

// ---- engine ------------------------------------------------------------

// launchWorkflowStep launches a template as step `idx` of a workflow run, tagging
// the run so the terminal hook can advance the pipeline.
func (s *Server) launchWorkflowStep(ctx context.Context, wfRunID string, idx int, t *model.Template, by string, vars map[string]any, invOverride, envOverride, vaultOverride string) (*model.Run, error) {
	run := s.buildRunFromTemplate(t, fmt.Sprintf(" ‹wf·%d›", idx+1), by, vars) // workflow vars → extra-vars
	run.WorkflowRunID = &wfRunID
	run.WorkflowStep = idx
	return s.launchTemplateRun(ctx, run, t, invOverride, envOverride, vaultOverride)
}

// startWorkflow creates a workflow run and launches the first runnable step.
func (s *Server) startWorkflow(ctx context.Context, wf *model.Workflow, by string, overrides map[string]any) (*model.WorkflowRun, error) {
	if len(wf.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}
	// Effective variables = the workflow's defaults, overridden at launch; injected
	// as extra-vars into every step.
	vars := map[string]any{}
	for k, v := range wf.Variables {
		vars[k] = v
	}
	for k, v := range overrides {
		vars[k] = v
	}
	wr := &model.WorkflowRun{
		WorkflowID: wf.ID, ProjectID: wf.ProjectID, Name: wf.Name,
		TriggeredBy: by, Status: model.WFStatusRunning, Variables: vars,
	}
	for _, step := range wf.Steps {
		cond := step.Condition
		if cond == "" {
			cond = model.WFCondOnSuccess
		}
		wr.Steps = append(wr.Steps, model.WorkflowRunStep{
			Name: step.Name, TemplateID: step.TemplateID, Condition: cond, When: step.When,
			InventoryID: step.InventoryID, EnvironmentID: step.EnvironmentID, VaultCredentialID: step.VaultCredentialID,
			Parallel: step.Parallel, Status: "pending",
		})
	}
	if err := s.store.CreateWorkflowRun(ctx, wr); err != nil {
		return nil, err
	}
	s.recordActivity(ctx, by, "workflow.started", wf.Name, "")
	s.events.Publish(map[string]any{"type": "workflow", "id": wr.ID})
	s.advanceFrom(ctx, wr, 0, false)
	return wr, nil
}

// advanceFrom launches the next runnable **wave** of steps (the next pending
// anchor plus any immediately-following steps marked Parallel), marking
// condition-skipped steps; or finalises the workflow run when none remain. The
// run-terminal hook calls this only when no step is still running.
func (s *Server) advanceFrom(ctx context.Context, wr *model.WorkflowRun, from int, failed bool) {
	launched := false
	for i := from; i < len(wr.Steps); i++ {
		st := &wr.Steps[i]
		if st.Status != "pending" {
			continue // already done / running / skipped
		}
		// After the wave anchor, only Parallel steps join the same wave; the first
		// non-parallel pending step ends the wave (runs in the next round).
		if launched && !st.Parallel {
			break
		}
		runnable := st.Condition == model.WFCondAlways ||
			(st.Condition == model.WFCondOnSuccess && !failed) ||
			(st.Condition == model.WFCondOnFailure && failed)
		if runnable && st.When != "" && !evalWhen(st.When, wr.Variables) {
			runnable = false
		}
		if !runnable {
			st.Status = "skipped"
			continue
		}
		t, terr := s.store.GetTemplate(ctx, st.TemplateID)
		if terr != nil {
			st.Status = model.StatusFailed
			failed = true
			continue
		}
		run, lerr := s.launchWorkflowStep(ctx, wr.ID, i, t, wr.TriggeredBy, wr.Variables, st.InventoryID, st.EnvironmentID, st.VaultCredentialID)
		if lerr != nil {
			st.Status = model.StatusFailed
			failed = true
			continue
		}
		st.RunID = run.ID
		st.Status = model.StatusRunning
		launched = true
	}
	if launched {
		_ = s.store.UpdateWorkflowRun(ctx, wr)
		s.events.Publish(map[string]any{"type": "workflow", "id": wr.ID})
		return // wave running — the terminal hook resumes once all of it finishes
	}
	if failed {
		wr.Status = model.WFStatusFailed
	} else {
		wr.Status = model.WFStatusSuccess
	}
	_ = s.store.UpdateWorkflowRun(ctx, wr)
	s.recordActivity(ctx, wr.TriggeredBy, "workflow."+wr.Status, wr.Name, "")
	s.events.Publish(map[string]any{"type": "workflow", "id": wr.ID})
}

// afterRunTerminal is the single run-terminal hook (wired to manager.onTerminal):
// advance any workflow this run belongs to, then forward the event to syslog.
func (s *Server) afterRunTerminal(run *model.Run, status string) {
	s.advanceWorkflow(run, status)
	s.forwardSyslog(run, status)
	s.forwardHTTPLog(run, status)
	s.longRunAlerted.Delete(run.ID) // run finished — free the long-run-alert dedup entry
}

// advanceWorkflow records the finished step's outcome and, once **every** step in
// the current wave has finished, launches the next wave (or finalises the run).
// Serialised by wfMu because a parallel wave's runs finish concurrently.
func (s *Server) advanceWorkflow(run *model.Run, status string) {
	if run.WorkflowRunID == nil || *run.WorkflowRunID == "" {
		return
	}
	s.wfMu.Lock()
	defer s.wfMu.Unlock()
	ctx := context.Background()
	wr, err := s.store.GetWorkflowRun(ctx, *run.WorkflowRunID)
	if err != nil || wr.Status != model.WFStatusRunning {
		return
	}
	if run.WorkflowStep < 0 || run.WorkflowStep >= len(wr.Steps) {
		return
	}
	wr.Steps[run.WorkflowStep].Status = status
	wr.Steps[run.WorkflowStep].RunID = run.ID
	// Merge any variables the step exported (aui_output.json) so later steps see them.
	s.mergeStepOutput(run.ID, wr)
	// If any step is still running, the current wave isn't done yet — just persist
	// this outcome and wait for the rest of the wave to finish.
	for i := range wr.Steps {
		if wr.Steps[i].Status == model.StatusRunning {
			_ = s.store.UpdateWorkflowRun(ctx, wr)
			s.events.Publish(map[string]any{"type": "workflow", "id": wr.ID})
			return
		}
	}
	failed := false
	for i := range wr.Steps {
		if wr.Steps[i].Status == model.StatusFailed || wr.Steps[i].Status == model.StatusCanceled {
			failed = true
		}
	}
	s.advanceFrom(ctx, wr, 0, failed) // scan from 0; skips done/skipped, launches the next wave
}

// workflowOutputFile is the conventional file a step writes (a flat JSON object)
// to export variables to subsequent steps.
const workflowOutputFile = "aui_output.json"

// mergeStepOutput reads a finished step's captured aui_output.json (if any) and
// merges its top-level keys into the workflow run's variables.
func (s *Server) mergeStepOutput(runID string, wr *model.WorkflowRun) {
	b, err := os.ReadFile(filepath.Join(s.artifactDir(runID), workflowOutputFile))
	if err != nil {
		return // no output file — nothing to merge
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil || len(out) == 0 {
		return
	}
	if wr.Variables == nil {
		wr.Variables = map[string]any{}
	}
	for k, v := range out {
		wr.Variables[k] = v
	}
}

// ---- handlers ----------------------------------------------------------

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	wfs, err := s.store.ListWorkflows(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if wfs == nil {
		wfs = []*model.Workflow{}
	}
	writeJSON(w, http.StatusOK, wfs)
}

// handleListAllWorkflows lists workflows across projects (optional ?projectId) —
// used by selectors like the schedule form.
func (s *Server) handleListAllWorkflows(w http.ResponseWriter, r *http.Request) {
	wfs, err := s.store.ListWorkflows(r.Context(), r.URL.Query().Get("projectId"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if wfs == nil {
		wfs = []*model.Workflow{}
	}
	writeJSON(w, http.StatusOK, wfs)
}

// validateCrossProjectSteps ensures the actor may RUN each step's template in
// its OWN project — so a cross-project step (a step whose template lives in a
// different project) can't smuggle a run into a project the user can't access.
// Same-project steps are already covered by the workflow's CapEdit gate.
func (s *Server) validateCrossProjectSteps(w http.ResponseWriter, r *http.Request, wfProjectID string, steps []model.WorkflowStep) bool {
	u := currentUser(r)
	for _, st := range steps {
		if st.TemplateID == "" {
			continue
		}
		t, err := s.store.GetTemplate(r.Context(), st.TemplateID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "step template not found: "+st.TemplateID)
			return false
		}
		if t.ProjectID == wfProjectID {
			continue
		}
		if !s.projectCaps(r, t.ProjectID, u)[model.CapRun] {
			writeErr(w, http.StatusForbidden, "no run access to the project of cross-project step template '"+t.Name+"'")
			return false
		}
	}
	return true
}

// handleListAllTemplates lists templates across every project the caller can RUN
// in (each hydrated with its project name) — powering the cross-project picker in
// the workflow step editor, so a step can target a template in another project.
func (s *Server) handleListAllTemplates(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	out := []*model.Template{}
	for _, p := range projects {
		if !s.projectCaps(r, p.ID, u)[model.CapRun] {
			continue
		}
		ts, terr := s.store.ListTemplates(r.Context(), p.ID)
		if terr != nil {
			continue
		}
		for _, t := range ts {
			t.ProjectName = p.Name
			out = append(out, t)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	if !s.requireProjectCap(w, r, pid, model.CapEdit) {
		return
	}
	var in struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Steps       []model.WorkflowStep `json:"steps"`
		Variables   map[string]string    `json:"variables"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if !s.validateCrossProjectSteps(w, r, pid, in.Steps) {
		return
	}
	wf := &model.Workflow{ProjectID: pid, Name: in.Name, Description: in.Description, Steps: in.Steps, Variables: in.Variables}
	if err := s.store.CreateWorkflow(r.Context(), wf); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "workflow.created", wf.Name, "")
	writeJSON(w, http.StatusCreated, wf)
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, wf.ProjectID, model.CapEdit) {
		return
	}
	// Snapshot the current definition before applying the edit (version history).
	if err := s.store.SnapshotWorkflowVersion(r.Context(), wf, s.actor(r.Context())); err != nil {
		s.log.Debug("workflow version snapshot failed", "err", err)
	}
	var in struct {
		Name        *string              `json:"name"`
		Description *string              `json:"description"`
		Steps       []model.WorkflowStep `json:"steps"`
		Variables   map[string]string    `json:"variables"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		wf.Name = *in.Name
	}
	if in.Description != nil {
		wf.Description = *in.Description
	}
	if in.Steps != nil {
		if !s.validateCrossProjectSteps(w, r, wf.ProjectID, in.Steps) {
			return
		}
		wf.Steps = in.Steps
	}
	if in.Variables != nil {
		wf.Variables = in.Variables
	}
	if err := s.store.UpdateWorkflow(r.Context(), wf); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "workflow.updated", wf.Name, "")
	writeJSON(w, http.StatusOK, wf)
}

// handleListWorkflowVersions returns a workflow's version history.
func (s *Server) handleListWorkflowVersions(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	versions, err := s.store.ListWorkflowVersions(r.Context(), wf.ID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// handleRollbackWorkflow restores a workflow to a past version. The current
// definition is first snapshotted (so a rollback is itself reversible).
func (s *Server) handleRollbackWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, wf.ProjectID, model.CapEdit) {
		return
	}
	v, err := s.store.GetWorkflowVersion(r.Context(), r.PathValue("versionId"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if v.WorkflowID != wf.ID {
		writeErr(w, http.StatusBadRequest, "version does not belong to this workflow")
		return
	}
	// Snapshot current (the rollback is reversible), then restore the version.
	_ = s.store.SnapshotWorkflowVersion(r.Context(), wf, s.actor(r.Context()))
	wf.Name, wf.Description, wf.Steps, wf.Variables = v.Name, v.Description, v.Steps, v.Variables
	if err := s.store.UpdateWorkflow(r.Context(), wf); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "workflow.rolledBack", wf.Name, "v"+strconv.Itoa(v.Version))
	writeJSON(w, http.StatusOK, wf)
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, wf.ProjectID, model.CapEdit) {
		return
	}
	if err := s.store.DeleteWorkflow(r.Context(), wf.ID); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "workflow.deleted", wf.Name, "")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, wf.ProjectID, model.CapEdit) {
		return
	}
	var in struct {
		Variables map[string]any `json:"variables"`
	}
	_ = decodeJSON(r, &in) // optional body: launch-time variable overrides
	wr, err := s.startWorkflow(r.Context(), wf, s.actor(r.Context()), in.Variables)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, wr)
}

func (s *Server) handleListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	runs, err := s.store.ListWorkflowRuns(r.Context(), q.Get("projectId"), q.Get("workflowId"), 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []*model.WorkflowRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	wr, err := s.store.GetWorkflowRun(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wr)
}

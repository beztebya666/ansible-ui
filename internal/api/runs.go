package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	pid := r.URL.Query().Get("projectId")
	if pid == "" {
		pid = projectScope(r)
	}
	runs, err := s.store.ListRuns(r.Context(), store.RunFilter{
		ProjectID: pid,
		Status:    r.URL.Query().Get("status"),
		Limit:     limit,
	})
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if run.RunnerID != "" {
		if rn, rerr := s.store.GetRunner(r.Context(), run.RunnerID); rerr == nil {
			run.RunnerName = rn.Name
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"run":    run,
		"active": s.manager.IsActive(run.ID),
	})
}

// handleApproveRun releases a run held for approval — project admin only. The
// approved run is then dispatched (respecting runner capacity / the queue).
func (s *Server) handleApproveRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, run.ProjectID, model.CapManage) {
		return
	}
	if run.Status != model.StatusAwaiting {
		writeErr(w, http.StatusConflict, "run is not awaiting approval")
		return
	}
	// Move it into the queue; the leader's drainer dispatches it (its launch
	// envelope is already persisted).
	if err := s.store.SetRunStatus(r.Context(), run.ID, model.StatusQueued); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	run.Status = model.StatusQueued
	s.recordActivity(r.Context(), s.actor(r.Context()), "run.approved", run.Name, run.App)
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	s.nudgeDrainer()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "approved"})
}

// handleRejectRun rejects a run held for approval — project admin only.
func (s *Server) handleRejectRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, run.ProjectID, model.CapManage) {
		return
	}
	if run.Status != model.StatusAwaiting {
		writeErr(w, http.StatusConflict, "run is not awaiting approval")
		return
	}
	_ = s.store.SetRunEnvelope(r.Context(), run.ID, nil)
	_ = s.store.FinishRun(r.Context(), run.ID, model.StatusCanceled, -1, model.RunStats{}, []byte("Rejected by "+s.actor(r.Context())+"\r\n"), nil)
	run.Status = model.StatusCanceled
	s.recordActivity(r.Context(), s.actor(r.Context()), "run.rejected", run.Name, run.App)
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rejected"})
}

// handleRunSecrets reveals a run's real secret extra-var values — admin only.
// The run record/log only ever store masked values; this decrypts the stashed
// blob on demand and audits who revealed them.
func (s *Server) handleRunSecrets(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	blob, err := s.store.GetRunSecretVars(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	out := map[string]any{}
	if len(blob) > 0 {
		if raw, derr := s.cipher.Decrypt(blob); derr == nil {
			_ = json.Unmarshal(raw, &out)
		}
	}
	if len(out) > 0 {
		s.recordActivity(r.Context(), s.actor(r.Context()), "run.secretsRevealed", r.PathValue("id"), "")
		s.recordSecretAccess(r.Context(), "", "run-secrets", "", "", "revealed secret vars of run "+r.PathValue("id"))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRunOutput(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.GetRunOutput(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(out)
}

// handleRunLog returns the stored output plus per-line completion timestamps (ms)
// for the structured, Semaphore-style log view (timestamp gutter + colors).
func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	out, times, err := s.store.GetRunLog(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if times == nil {
		times = []int64{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": string(out), "times": times})
}

// handleCreateRun launches an ad-hoc run.
func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProjectID         string         `json:"projectId"`
		App               string         `json:"app"`
		Action            string         `json:"action"`
		Playbook          string         `json:"playbook"`
		InventoryID       string         `json:"inventoryId"`
		Name              string         `json:"name"`
		Limit             string         `json:"limit"`
		Tags              string         `json:"tags"`
		SkipTags          string         `json:"skipTags"`
		ExtraVars         map[string]any `json:"extraVars"`
		Check             bool           `json:"check"`
		Diff              bool           `json:"diff"`
		Verbosity         int            `json:"verbosity"`
		CliArgs           []string       `json:"cliArgs"`
		Workspace         string         `json:"workspace"`
		AutoApprove       bool           `json:"autoApprove"`
		VaultCredentialID string         `json:"vaultCredentialId"`
		Vaults            []string       `json:"vaults"`
		EnvironmentID     string         `json:"environmentId"`
		Timezone          string         `json:"timezone"`
		RunnerTag         string         `json:"runnerTag"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.ProjectID == "" || in.Playbook == "" {
		writeErr(w, http.StatusBadRequest, "projectId and playbook are required")
		return
	}
	if !s.requireProjectCap(w, r, in.ProjectID, model.CapRun) {
		return
	}
	name := in.Name
	if name == "" {
		name = filepath.Base(in.Playbook)
	}
	run := &model.Run{
		ProjectID:   in.ProjectID,
		App:         in.App,
		Action:      in.Action,
		Name:        name,
		Playbook:    in.Playbook,
		Limit:       in.Limit,
		Tags:        in.Tags,
		SkipTags:    in.SkipTags,
		ExtraVars:   in.ExtraVars,
		Check:       in.Check,
		Diff:        in.Diff,
		Verbosity:   in.Verbosity,
		CliArgs:     in.CliArgs,
		Workspace:   in.Workspace,
		AutoApprove: in.AutoApprove,
	}
	vaultIDs := in.Vaults
	if in.VaultCredentialID != "" {
		vaultIDs = append(vaultIDs, in.VaultCredentialID)
	}
	if err := s.launchRun(r.Context(), run, in.InventoryID, vaultIDs, in.EnvironmentID, in.Timezone, "", in.RunnerTag, false); err != nil {
		s.launchError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

// applyTemplateArtifact implements Build/Deploy template types:
//   - build  → assigns the run an incrementing build number (#N) as its artifact.
//   - deploy → finds the latest successful build of its build template and passes
//     that artifact (version, commit, run id) into the run as extra-vars, so the
//     deploy playbook knows exactly what it's shipping.
func (s *Server) applyTemplateArtifact(ctx context.Context, run *model.Run, t *model.Template) {
	switch t.Type {
	case model.TemplateTypeBuild:
		if n, err := s.store.NextBuildNumber(ctx, t.ID); err == nil {
			run.Version = "#" + strconv.Itoa(n)
		}
	case model.TemplateTypeDeploy:
		if t.BuildTemplateID == nil || *t.BuildTemplateID == "" {
			return
		}
		b, err := s.store.LatestSuccessfulBuild(ctx, *t.BuildTemplateID)
		if err != nil {
			return // nothing built yet — deploy runs without an artifact
		}
		run.Version = b.Version
		merged := map[string]any{}
		for k, v := range run.ExtraVars {
			merged[k] = v
		}
		merged["build_version"] = b.Version
		merged["build_commit"] = b.Commit
		merged["build_run_id"] = b.ID
		run.ExtraVars = merged
		if b.Version != "" {
			run.Name = run.Name + " → " + b.Version
		}
	}
}

// handleRunTemplate launches a run from a saved template (with optional overrides).
func (s *Server) handleRunTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, t.ProjectID, model.CapRun) {
		return
	}
	var ov struct {
		InventoryID       *string        `json:"inventoryId"`
		VaultCredentialID *string        `json:"vaultCredentialId"`
		EnvironmentID     *string        `json:"environmentId"`
		Timezone          *string        `json:"timezone"`
		Branch            *string        `json:"branch"`
		Limit             *string        `json:"limit"`
		Tags              *string        `json:"tags"`
		SkipTags          *string        `json:"skipTags"`
		Vaults            *[]string      `json:"vaults"`
		CliArgs           *[]string      `json:"cliArgs"`
		ExtraVars         map[string]any `json:"extraVars"`
		Check             *bool          `json:"check"`
		Diff              *bool          `json:"diff"`
		Verbosity         *int           `json:"verbosity"`
	}
	if err := decodeOptional(r, &ov); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	run := &model.Run{
		ProjectID:     t.ProjectID,
		TemplateID:    &t.ID,
		App:           t.App,
		Action:        t.Action,
		Name:          t.Name,
		Playbook:      t.Playbook,
		Limit:         t.Limit,
		Tags:          t.Tags,
		SkipTags:      t.SkipTags,
		ExtraVars:     t.ExtraVars,
		Check:         t.Check,
		Diff:          t.Diff,
		Verbosity:     t.Verbosity,
		CliArgs:       t.CliArgs,
		Workspace:     t.Workspace,
		ArtifactPaths: t.ArtifactPaths,
		AutoApprove:   t.AutoApprove,
	}
	if ov.Limit != nil {
		run.Limit = *ov.Limit
	}
	if ov.Tags != nil {
		run.Tags = *ov.Tags
	}
	if ov.SkipTags != nil {
		run.SkipTags = *ov.SkipTags
	}
	if ov.ExtraVars != nil {
		run.ExtraVars = ov.ExtraVars
	}
	if ov.Check != nil {
		run.Check = *ov.Check
	}
	if ov.Diff != nil {
		run.Diff = *ov.Diff
	}
	if ov.Verbosity != nil {
		run.Verbosity = *ov.Verbosity
	}
	if ov.CliArgs != nil {
		run.CliArgs = *ov.CliArgs
	}
	branchOverride := ""
	if ov.Branch != nil {
		branchOverride = *ov.Branch
	}

	inventoryID := ""
	if t.InventoryID != nil {
		inventoryID = *t.InventoryID
	}
	if ov.InventoryID != nil {
		inventoryID = *ov.InventoryID
	}
	if !templateInventoryAllowed(t, inventoryID) {
		writeErr(w, http.StatusBadRequest, "inventory not allowed for this template")
		return
	}

	// Multi-vault: an explicit launch-time selection (ov.Vaults) replaces the
	// template's list; otherwise use the template's vaults + legacy single.
	var vaultIDs []string
	if ov.Vaults != nil {
		vaultIDs = append([]string{}, (*ov.Vaults)...)
	} else {
		vaultIDs = append([]string{}, t.Vaults...)
		if t.VaultCredentialID != nil && *t.VaultCredentialID != "" {
			vaultIDs = append(vaultIDs, *t.VaultCredentialID)
		}
	}
	if ov.VaultCredentialID != nil && *ov.VaultCredentialID != "" {
		vaultIDs = append(vaultIDs, *ov.VaultCredentialID)
	}

	environmentID := ""
	if t.EnvironmentID != nil {
		environmentID = *t.EnvironmentID
	}
	if ov.EnvironmentID != nil {
		environmentID = *ov.EnvironmentID
	}

	tz := ""
	if ov.Timezone != nil {
		tz = *ov.Timezone
	}
	s.applyTemplateArtifact(r.Context(), run, t)
	if err := s.launchRun(r.Context(), run, inventoryID, vaultIDs, environmentID, tz, branchOverride, t.RunnerTag, t.RequiresApproval); err != nil {
		s.launchError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.manager.IsActive(id) {
		// A run still waiting in the dispatch queue can be canceled before it starts.
		if s.cancelQueued(id) {
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceled"})
			return
		}
		// Not streaming on THIS replica: if it's still in progress it's running on
		// another replica — flag it in Postgres so the owning replica cancels it.
		if run, err := s.store.GetRun(r.Context(), id); err == nil && !model.IsTerminalStatus(run.Status) {
			if cerr := s.store.RequestCancel(r.Context(), id); cerr == nil {
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceling"})
				return
			}
		}
		writeErr(w, http.StatusConflict, "run is not active")
		return
	}
	s.manager.Control(id, model.Frame{Type: model.FrameCancel})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceling"})
}

// handleDeleteRun removes a single run from history. An active run must be
// canceled first (you can't delete something that's still streaming).
func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.manager.IsActive(id) {
		writeErr(w, http.StatusConflict, "run is active — cancel it first")
		return
	}
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if err := s.store.DeleteRun(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	_ = os.RemoveAll(s.artifactDir(id)) // drop captured files from disk (DB rows cascade)
	s.recordActivity(r.Context(), "", "run.deleted", run.Name, run.App)
	s.events.Publish(map[string]any{"type": "run.updated"})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": 1})
}

// handleClearRuns wipes finished run history (admin only). Optional ?projectId
// scopes it to one project; active/pending runs are always preserved.
func (s *Server) handleClearRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ids, err := s.store.ClearRuns(r.Context(), r.URL.Query().Get("projectId"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	for _, id := range ids {
		_ = os.RemoveAll(s.artifactDir(id)) // drop captured artifacts from disk too
	}
	s.recordActivity(r.Context(), "", "run.cleared", strconv.Itoa(len(ids))+" runs", "")
	s.events.Publish(map[string]any{"type": "run.updated"})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": len(ids)})
}

// runTemplateDefaults launches a run from a template's stored defaults (used by
// the scheduler and webhooks). nameSuffix labels the trigger source; extra adds
// extra-vars on top of the template's (e.g. a webhook payload).
func (s *Server) runTemplateDefaults(ctx context.Context, t *model.Template, nameSuffix string, extra map[string]any) (*model.Run, error) {
	return s.runTemplateBy(ctx, t, nameSuffix, "", extra)
}

// runTemplateBy is runTemplateDefaults with an explicit actor (scheduler/webhook).
func (s *Server) runTemplateBy(ctx context.Context, t *model.Template, nameSuffix, by string, extra map[string]any) (*model.Run, error) {
	return s.launchTemplateRun(ctx, s.buildRunFromTemplate(t, nameSuffix, by, extra), t, "", "", "")
}

// buildRunFromTemplate materialises a run object from a template's stored config
// (merging extra vars on top), without launching it.
func (s *Server) buildRunFromTemplate(t *model.Template, nameSuffix, by string, extra map[string]any) *model.Run {
	vars := t.ExtraVars
	if len(extra) > 0 {
		vars = map[string]any{}
		for k, v := range t.ExtraVars {
			vars[k] = v
		}
		for k, v := range extra {
			vars[k] = v
		}
	}
	return &model.Run{
		ProjectID:     t.ProjectID,
		TemplateID:    &t.ID,
		TriggeredBy:   by,
		App:           t.App,
		Action:        t.Action,
		Name:          t.Name + nameSuffix,
		Playbook:      t.Playbook,
		Limit:         t.Limit,
		Tags:          t.Tags,
		SkipTags:      t.SkipTags,
		ExtraVars:     vars,
		Check:         t.Check,
		Diff:          t.Diff,
		Verbosity:     t.Verbosity,
		CliArgs:       t.CliArgs,
		Workspace:     t.Workspace,
		ArtifactPaths: t.ArtifactPaths,
		AutoApprove:   t.AutoApprove,
	}
}

// templateInventoryAllowed enforces a template's "allowed inventories" list: when
// InventoryIDs is non-empty, a run may only use the template's default inventory
// or one of that curated set (so an API client can't bypass the UI's constrained
// picker). An empty list means no restriction (backward-compatible).
func templateInventoryAllowed(t *model.Template, invID string) bool {
	if len(t.InventoryIDs) == 0 || invID == "" {
		return true
	}
	if t.InventoryID != nil && *t.InventoryID == invID {
		return true // the default is always permitted
	}
	for _, id := range t.InventoryIDs {
		if id == invID {
			return true
		}
	}
	return false
}

// launchTemplateRun resolves the template's vault/inventory/env + artifact and
// launches the given (already-built) run.
func (s *Server) launchTemplateRun(ctx context.Context, run *model.Run, t *model.Template, invOverride, envOverride, vaultOverride string) (*model.Run, error) {
	vaultIDs := append([]string{}, t.Vaults...)
	if t.VaultCredentialID != nil && *t.VaultCredentialID != "" {
		vaultIDs = append(vaultIDs, *t.VaultCredentialID)
	}
	if vaultOverride != "" { // workflow per-step vault override — replaces the template's vault(s)
		vaultIDs = []string{vaultOverride}
	}
	inventoryID := derefStr(t.InventoryID)
	if invOverride != "" { // workflow per-step inventory override
		inventoryID = invOverride
	}
	if !templateInventoryAllowed(t, inventoryID) {
		return nil, fmt.Errorf("inventory not allowed for this template")
	}
	environmentID := derefStr(t.EnvironmentID)
	if envOverride != "" { // workflow per-step environment override
		environmentID = envOverride
	}
	s.applyTemplateArtifact(ctx, run, t)
	if err := s.launchRun(ctx, run, inventoryID, vaultIDs, environmentID, "", "", t.RunnerTag, t.RequiresApproval); err != nil {
		return nil, err
	}
	return run, nil
}

// secretMask replaces a secret extra-var's value in everything that is persisted
// or displayed (run.ExtraVars, the echoed command, the UI) — the operator sees
// which secret is in play without the raw value ever being stored or logged.
const secretMask = "••••••"

// launchRun resolves a working directory, inventory and environment, persists
// the pending run, then hands it to the manager to execute.
func (s *Server) launchRun(ctx context.Context, run *model.Run, inventoryID string, vaultCredIDs []string, environmentID, tz, branch, runnerTag string, requiresApproval bool) error {
	project, err := s.store.GetProject(ctx, run.ProjectID)
	if err != nil {
		return err
	}
	// Global concurrency cap: block a launch when the whole system already has
	// tasks.max_parallel runs queued/active (0 = unlimited).
	if maxPar := s.intSetting(ctx, settingMaxParallel); maxPar > 0 {
		if active, cerr := s.store.CountActiveRuns(ctx); cerr == nil && active >= maxPar {
			return errNotReady(fmt.Sprintf("the system is at its global max parallel runs (%d) — try again shortly", maxPar))
		}
	}
	// Per-template concurrency cap: block a launch when the template already has
	// maxConcurrent runs queued/active (0 = unlimited). Best-effort (count then create).
	if run.TemplateID != nil {
		if tpl, terr := s.store.GetTemplate(ctx, *run.TemplateID); terr == nil && tpl.MaxConcurrent > 0 {
			if active, cerr := s.store.CountActiveRunsByTemplate(ctx, *run.TemplateID); cerr == nil && active >= tpl.MaxConcurrent {
				return errNotReady(fmt.Sprintf("template is at its max concurrent runs (%d)", tpl.MaxConcurrent))
			}
		}
	}
	// Assign the id up front (CreateRun keeps a preset id) so anything keyed by it
	// below — e.g. the transient static-inventory filename — is unique per run.
	if run.ID == "" {
		run.ID = store.NewID("run")
	}
	if run.App == "" {
		run.App = model.AppAnsible
	}
	if run.TriggeredBy == "" {
		run.TriggeredBy = s.actor(ctx)
	}

	// Inventory→runner affinity + SSH connection credentials. An inventory can pin
	// its runs to a runner advertising a tag (its hosts may only be reachable from
	// that runner — wins over the template/run tag), and can carry Key Store SSH
	// credentials whose keys ansible uses to connect to its hosts.
	var sshConnKeys []string
	var sshConnUser string
	if inventoryID != "" {
		if inv, ierr := s.store.GetInventory(ctx, inventoryID); ierr == nil {
			if strings.TrimSpace(inv.RunnerTag) != "" {
				runnerTag = strings.TrimSpace(inv.RunnerTag)
			}
			for _, cid := range inv.ConnCredentialIDs {
				if sec, serr := s.credentialSecret(ctx, cid); serr == nil && sec != nil && strings.TrimSpace(sec.SSHPrivateKey) != "" {
					sshConnKeys = append(sshConnKeys, sec.SSHPrivateKey)
					if cred, cerr := s.store.GetCredential(ctx, cid); cerr == nil && sshConnUser == "" {
						sshConnUser = cred.Login
					}
					s.recordSecretAccess(ctx, run.TriggeredBy, "credential-use", cid, "", "ssh connection key used by run "+run.ID)
				}
			}
		}
	}

	// A tagged run targets a remote runner pool; local-directory projects (files
	// only on the control plane) can't run there — reject early and clearly.
	gitBacked := project.RepositoryID != nil && *project.RepositoryID != ""
	remoteDispatch := runnerTag != "" && runnerTag != "default"
	if remoteDispatch && !gitBacked {
		return errNotReady("local-directory projects can only run on the built-in runner — back the project with a git repository to use a remote runner pool")
	}

	// Resolve the application config (built-in or custom). Disabled apps are blocked;
	// custom apps run an arbitrary binary; built-in base args fold into cliArgs.
	var customBin string
	var customArgs []string
	if appCfg, aerr := s.store.GetApplication(ctx, run.App); aerr == nil {
		if !appCfg.Active {
			return errNotReady("application '" + run.App + "' is disabled")
		}
		if appCfg.Kind == model.AppKindCustom && appCfg.Bin != "" {
			customBin = appCfg.Bin
			customArgs = appCfg.Args
		} else if len(appCfg.Args) > 0 {
			run.CliArgs = append(append([]string{}, appCfg.Args...), run.CliArgs...)
		}
	}

	// Environment: lowest-precedence extra-vars + process env vars. secretVars holds
	// resolved secret extra-vars (env var-secrets + external secrets) — they go to
	// the runner's spec but are NEVER merged into the persisted/echoed run.ExtraVars.
	envExtra := map[string]any{}
	envProcess := map[string]string{}
	secretVars := map[string]any{}
	if environmentID != "" {
		envObj, err := s.store.GetEnvironment(ctx, environmentID)
		if err != nil {
			return err
		}
		envExtra = envObj.ExtraVars
		for k, v := range envObj.EnvVars {
			envProcess[k] = v
		}
		// Decrypted secrets: var-type → extra-vars (secretVars), env-type → process env.
		for _, sec := range s.envSecretsFor(ctx, envObj) {
			if sec.Type == model.EnvSecretVar {
				secretVars[sec.Name] = sec.Value
			} else {
				envProcess[sec.Name] = sec.Value
			}
		}
		// External secrets: fetched fresh from their backend (Vault) at launch.
		// A resolution failure aborts the launch — we never run without a secret
		// the operator declared required.
		for _, ref := range envObj.ExternalSecrets {
			if ref.Name == "" || ref.Backend == "" {
				continue
			}
			val, rerr := s.resolveExternalSecret(ctx, ref)
			if rerr != nil {
				return errNotReady("external secret " + ref.Name + ": " + rerr.Error())
			}
			if ref.AsVar {
				secretVars[ref.Name] = val
			} else {
				envProcess[ref.Name] = val
			}
		}
		run.EnvironmentID = &environmentID
	}

	// Repo-backed projects: pull the latest commit before every run. The built-in
	// sync keeps the control plane's view fresh (file browser, detection); the
	// per-run repoSpec (captured below) re-syncs onto a remote runner at dispatch.
	var repoSpec model.GitSyncSpec
	var repoURL, repoBranch, commitMsg string
	if gitBacked {
		repo, err := s.store.GetRepository(ctx, *project.RepositoryID)
		if err != nil {
			return err
		}
		// Per-run branch override (the "branch" launch prompt).
		if branch != "" {
			repo.Branch = branch
		}
		res, serr := s.syncRepository(ctx, repo)
		if serr != nil {
			return errNotReady("git sync failed: " + serr.Error())
		}
		if res.Error != "" {
			return errNotReady("git sync failed: " + res.Error)
		}
		run.Commit = res.Commit
		repoURL, repoBranch, commitMsg = repo.GitURL, repo.Branch, res.Message
		repoSpec = s.repoSyncSpec(ctx, repo)
	}

	projectDir := s.projectDir(project)
	if _, err := os.Stat(projectDir); err != nil {
		return errNotReady("project directory missing on disk (sync the repository?)")
	}

	// Inventory only applies to Ansible runs.
	inventoryArg := ""
	inventoryName := ""             // exposed to the play as {{ inventory_name }}
	cloudEnv := map[string]string{} // provider creds for a cloud inventory (process env)
	if run.App == model.AppAnsible {
		if inventoryID != "" {
			inv, err := s.store.GetInventory(ctx, inventoryID)
			if err != nil {
				return err
			}
			inventoryName = inv.Name
			switch inv.Type {
			case model.InventoryCloud:
				// Turnkey cloud inventory: generate the plugin config + pass the
				// provider credentials as process env (never into extra-vars/log).
				cfg, cenv, gerr := s.cloudInventory(ctx, inv)
				if gerr != nil {
					return errNotReady(gerr.Error())
				}
				stale, _ := filepath.Glob(filepath.Join(projectDir, ".aui-inv-*"))
				for _, old := range stale {
					_ = os.Remove(old)
				}
				// The filename must end in "<plugin>.yml" so ansible loads the plugin.
				invName := ".aui-inv-" + run.ID + "." + inv.Provider + ".yml"
				if err := os.WriteFile(filepath.Join(projectDir, invName), []byte(cfg), 0o644); err != nil {
					return err
				}
				inventoryArg = invName
				for k, v := range cenv {
					cloudEnv[k] = v
				}
			case model.InventoryURL:
				// Fetch the inventory from an HTTP(S) URL at launch, then treat it
				// like a static inventory (written beside the project's group_vars).
				body, ferr := fetchURLInventory(ctx, inv.Content)
				if ferr != nil {
					return errNotReady("fetch inventory URL: " + ferr.Error())
				}
				stale, _ := filepath.Glob(filepath.Join(projectDir, ".aui-inv-*"))
				for _, old := range stale {
					_ = os.Remove(old)
				}
				invName := ".aui-inv-" + run.ID
				if err := os.WriteFile(filepath.Join(projectDir, invName), body, 0o644); err != nil {
					return err
				}
				inventoryArg = invName
			case model.InventoryFile, model.InventoryDynamic:
				// Content is a project-relative path; ansible resolves it from the
				// working dir. Dynamic inventories must be executable to be invoked
				// as scripts (otherwise ansible parses them as a static file).
				inventoryArg = inv.Content
				if inv.Type == model.InventoryDynamic {
					_ = os.Chmod(filepath.Join(projectDir, filepath.FromSlash(inv.Content)), 0o755)
				}
			default: // static — write the inline content into the PROJECT dir (not
				// /data/runs) so ansible discovers the project's group_vars/host_vars,
				// which live beside it; otherwise they'd silently not load. A transient
				// dotfile keyed by run id; stale ones from finished runs are swept first.
				stale, _ := filepath.Glob(filepath.Join(projectDir, ".aui-inv-*"))
				for _, old := range stale {
					_ = os.Remove(old)
				}
				invName := ".aui-inv-" + run.ID
				if err := os.WriteFile(filepath.Join(projectDir, invName), []byte(inv.Content), 0o644); err != nil {
					return err
				}
				inventoryArg = invName // relative to the project working dir
			}
		} else if _, err := os.Stat(filepath.Join(projectDir, "inventory.ini")); err == nil {
			inventoryArg = "inventory.ini"
		}
	}

	env := map[string]string{"HOME": "/tmp"}
	if _, err := os.Stat(filepath.Join(projectDir, "ansible.cfg")); err == nil {
		env["ANSIBLE_CONFIG"] = filepath.Join(projectDir, "ansible.cfg")
	}
	// Private galaxy / Automation Hub auth: inject ANSIBLE_GALAXY_SERVER_* so the
	// runner's `ansible-galaxy install -r requirements.yml` authenticates.
	for k, v := range s.galaxyEnv(ctx) {
		env[k] = v
	}
	if tz != "" {
		env["TZ"] = tz // run timestamps follow the operator's timezone
	}
	for k, v := range envProcess {
		env[k] = v
	}
	for k, v := range cloudEnv {
		env[k] = v
	}

	// Merge extra-vars: environment (lowest) then run/template/survey (wins).
	merged := map[string]any{}
	for k, v := range envExtra {
		merged[k] = v
	}
	for k, v := range run.ExtraVars {
		merged[k] = v
	}
	// The spec sent to the runner carries the REAL secret values (executed via a
	// transient file, never the command line). run.ExtraVars — persisted, echoed
	// in the command and shown in the UI — keeps non-secret values in full but
	// MASKS each secret: the operator sees WHICH secret is applied, never its value.
	specVars := map[string]any{}
	for k, v := range merged {
		specVars[k] = v
	}
	for k, v := range secretVars {
		specVars[k] = v
	}
	for k := range secretVars {
		merged[k] = secretMask
	}
	// Expose the inventory name to the play as {{ inventory_name }} (Semaphore parity:
	// "inventory in task context"). Don't clobber a user-set var of the same name.
	if inventoryName != "" {
		if _, ok := merged["inventory_name"]; !ok {
			merged["inventory_name"] = inventoryName
			specVars["inventory_name"] = inventoryName
		}
	}
	run.ExtraVars = merged

	// Stash the real secret values (AES-encrypted) so an admin can reveal them on
	// the run later; the plaintext is never persisted.
	if len(secretVars) > 0 {
		if raw, merr := json.Marshal(secretVars); merr == nil {
			if blob, eerr := s.cipher.Encrypt(raw); eerr == nil {
				run.SecretVarsBlob = blob
			}
		}
	}

	// Vault passwords from Key Store credentials (multi-vault).
	var vaultPasswords []string
	for _, id := range vaultCredIDs {
		if id == "" {
			continue
		}
		if sec, serr := s.credentialSecret(ctx, id); serr == nil && sec != nil {
			pw := sec.VaultPassword
			if pw == "" {
				pw = sec.Password
			}
			if pw != "" {
				vaultPasswords = append(vaultPasswords, pw)
			}
			credName := ""
			if c, cerr := s.store.GetCredential(ctx, id); cerr == nil && c != nil {
				credName = c.Name
			}
			s.recordSecretAccess(ctx, run.TriggeredBy, "credential-use", id, credName, "vault password used by run "+run.ID)
		}
	}

	// Terraform/OpenTofu HTTP state backend (template-level): the runner writes a
	// backend override + inits with -backend-config=address=… so state lives at the
	// configured URL instead of locally.
	tfBackend := ""
	if model.IsInfraApp(run.App) && run.TemplateID != nil {
		if tpl, terr := s.store.GetTemplate(ctx, *run.TemplateID); terr == nil && tpl != nil {
			tfBackend = tpl.TfBackend
		}
	}

	spec := model.ExecSpec{
		RunID:          run.ID,
		App:            run.App,
		AppBin:         customBin,
		AppArgs:        customArgs,
		Action:         run.Action,
		Dir:            projectDir,
		Playbook:       run.Playbook,
		Inventory:      inventoryArg,
		Limit:          run.Limit,
		Tags:           run.Tags,
		SkipTags:       run.SkipTags,
		ExtraVars:      specVars,
		Check:          run.Check,
		Diff:           run.Diff,
		Verbosity:      run.Verbosity,
		CliArgs:        run.CliArgs,
		Workspace:      run.Workspace,
		AutoApprove:    run.AutoApprove,
		Env:            env,
		Cols:           120,
		Rows:           34,
		GalaxyInstall:  run.App == model.AppAnsible,
		GalaxyArgs:     s.galaxyArgs(ctx),
		SSHPrivateKeys: sshConnKeys,
		SSHUser:        sshConnUser,
		RepoURL:        repoURL,
		RepoBranch:     repoBranch,
		CommitMsg:      commitMsg,
		VaultPasswords: vaultPasswords,
		ArtifactPaths:  run.ArtifactPaths,                         // runner captures + streams these back
		TimeoutSec:     s.intSetting(ctx, settingMaxTaskDuration), // 0 = unlimited; runner cancels on overrun
		TfBackend:      tfBackend,
	}
	// Workflow steps may export variables to later steps by writing aui_output.json
	// in the working dir — capture it like an artifact so the engine can merge it.
	if run.WorkflowRunID != nil {
		spec.ArtifactPaths = append(append([]string{}, spec.ArtifactPaths...), workflowOutputFile)
	}

	// The launch envelope carries everything the leader's drainer needs to dispatch
	// this run on any replica (the workspace-sync spec + the exec spec). It is
	// persisted AES-encrypted; the drainer materialises the workspace (git-backed
	// remote runners) and starts the PTY in s.dispatchRun.
	envelope := launchEnvelope{Spec: spec, RepoSpec: repoSpec, GitBacked: gitBacked, Tag: runnerTag}
	if requiresApproval {
		return s.holdForApproval(ctx, run, envelope)
	}
	return s.enqueueRun(ctx, run, envelope)
}

type notReadyError struct{ msg string }

func (e notReadyError) Error() string { return e.msg }
func errNotReady(m string) error      { return notReadyError{m} }

func (s *Server) launchError(w http.ResponseWriter, err error) {
	var nre notReadyError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "project, template or inventory not found")
	case errors.As(err, &nre):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		s.log.Error("launch failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "failed to launch run: "+err.Error())
	}
}

func decodeOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// fetchURLInventory downloads an inventory from an HTTP(S) URL at launch (30s
// timeout, 5 MB cap) for an InventoryURL source.
func fetchURLInventory(ctx context.Context, rawURL string) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("inventory URL is empty")
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("inventory body is empty")
	}
	return body, nil
}

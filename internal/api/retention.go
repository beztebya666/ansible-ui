package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	settingRetentionRuns         = "retention.runs_days"
	settingRetentionAudit        = "retention.audit_days"
	settingNonadminCreateProject = "flags.nonadmin_create_project"
	settingMaxTaskDuration       = "tasks.max_duration_sec" // 0 = unlimited
	settingMaxParallel           = "tasks.max_parallel"     // global concurrent-run cap; 0 = unlimited
)

// intSetting reads an integer app-setting (default 0).
func (s *Server) intSetting(ctx context.Context, key string) int {
	v, ok, err := s.store.GetSetting(ctx, key)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	if n < 0 {
		n = 0
	}
	return n
}

// handleGetTaskSettings returns task limits (admin).
func (s *Server) handleGetTaskSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"maxDurationSec":  s.intSetting(r.Context(), settingMaxTaskDuration),
		"longRunAlertSec": s.intSetting(r.Context(), settingLongRunAlert),
		"maxParallel":     s.intSetting(r.Context(), settingMaxParallel),
	})
}

// handleSetTaskSettings updates task limits (admin). maxDurationSec 0 = unlimited.
func (s *Server) handleSetTaskSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		MaxDurationSec  int `json:"maxDurationSec"`
		LongRunAlertSec int `json:"longRunAlertSec"`
		MaxParallel     int `json:"maxParallel"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.MaxDurationSec < 0 {
		in.MaxDurationSec = 0
	}
	if in.LongRunAlertSec < 0 {
		in.LongRunAlertSec = 0
	}
	if in.MaxParallel < 0 {
		in.MaxParallel = 0
	}
	_ = s.store.SetSetting(r.Context(), settingMaxTaskDuration, strconv.Itoa(in.MaxDurationSec))
	_ = s.store.SetSetting(r.Context(), settingLongRunAlert, strconv.Itoa(in.LongRunAlertSec))
	_ = s.store.SetSetting(r.Context(), settingMaxParallel, strconv.Itoa(in.MaxParallel))
	s.recordActivity(r.Context(), s.actor(r.Context()), "tasks.settingsUpdated",
		fmt.Sprintf("maxDuration=%ds longRunAlert=%ds maxParallel=%d", in.MaxDurationSec, in.LongRunAlertSec, in.MaxParallel), "")
	writeJSON(w, http.StatusOK, map[string]int{"maxDurationSec": in.MaxDurationSec, "longRunAlertSec": in.LongRunAlertSec, "maxParallel": in.MaxParallel})
}

// boolSetting reads a boolean app-setting (default false).
func (s *Server) boolSetting(ctx context.Context, key string) bool {
	v, ok, err := s.store.GetSetting(ctx, key)
	return err == nil && ok && v == "true"
}

// handleGetFlags returns the feature flags (admin).
func (s *Server) handleGetFlags(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"nonadminCreateProject": s.boolSetting(r.Context(), settingNonadminCreateProject),
	})
}

// handleSetFlags updates the feature flags (admin).
func (s *Server) handleSetFlags(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		NonadminCreateProject bool `json:"nonadminCreateProject"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	_ = s.store.SetSetting(r.Context(), settingNonadminCreateProject, boolStr(in.NonadminCreateProject))
	s.recordActivity(r.Context(), s.actor(r.Context()), "flags.updated", "nonadminCreateProject="+boolStr(in.NonadminCreateProject), "")
	writeJSON(w, http.StatusOK, map[string]bool{"nonadminCreateProject": in.NonadminCreateProject})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// retentionDays reads an integer app-setting (0 / unset = keep forever).
func (s *Server) retentionDays(ctx context.Context, key string) int {
	v, ok, err := s.store.GetSetting(ctx, key)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	if n < 0 {
		n = 0
	}
	return n
}

// evaluateRetention sweeps runs + audit older than the configured retention. A
// no-op when both are 0. Called on boot and hourly by the scheduler loop.
func (s *Server) evaluateRetention(ctx context.Context) {
	if d := s.retentionDays(ctx, settingRetentionRuns); d > 0 {
		if ids, err := s.store.DeleteRunsOlderThan(ctx, d); err != nil {
			s.log.Warn("retention: run sweep failed", "err", err)
		} else if len(ids) > 0 {
			for _, id := range ids {
				_ = os.RemoveAll(s.artifactDir(id))
			}
			s.log.Info("retention: swept runs", "count", len(ids), "olderThanDays", d)
		}
	}
	if d := s.retentionDays(ctx, settingRetentionAudit); d > 0 {
		if n, err := s.store.DeleteActivityOlderThan(ctx, d); err != nil {
			s.log.Warn("retention: audit sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention: swept audit", "count", n, "olderThanDays", d)
		}
		// The notification delivery log is audit data too — trim it on the same policy.
		if n, err := s.store.DeleteNotificationLogsOlderThan(ctx, d); err != nil {
			s.log.Warn("retention: notification-log sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention: swept notification log", "count", n, "olderThanDays", d)
		}
		// Same for the secret-access audit trail.
		if n, err := s.store.DeleteSecretAccessLogsOlderThan(ctx, d); err != nil {
			s.log.Warn("retention: secret-access sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention: swept secret-access log", "count", n, "olderThanDays", d)
		}
	}
}

// handleGetRetention returns the current retention policy (admin).
func (s *Server) handleGetRetention(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"runsDays":  s.retentionDays(r.Context(), settingRetentionRuns),
		"auditDays": s.retentionDays(r.Context(), settingRetentionAudit),
	})
}

// handleSetRetention updates the retention policy (admin). 0 = keep forever.
func (s *Server) handleSetRetention(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		RunsDays  int `json:"runsDays"`
		AuditDays int `json:"auditDays"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.RunsDays < 0 || in.AuditDays < 0 {
		writeErr(w, http.StatusBadRequest, "retention days cannot be negative")
		return
	}
	_ = s.store.SetSetting(r.Context(), settingRetentionRuns, strconv.Itoa(in.RunsDays))
	_ = s.store.SetSetting(r.Context(), settingRetentionAudit, strconv.Itoa(in.AuditDays))
	s.recordActivity(r.Context(), s.actor(r.Context()), "retention.updated",
		"runs="+strconv.Itoa(in.RunsDays)+"d audit="+strconv.Itoa(in.AuditDays)+"d", "")
	go s.evaluateRetention(context.Background()) // apply immediately
	writeJSON(w, http.StatusOK, map[string]int{"runsDays": in.RunsDays, "auditDays": in.AuditDays})
}

// handleExportAudit streams the full audit log as CSV (admin, for compliance).
func (s *Server) handleExportAudit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-`+time.Now().Format("20060102")+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"time", "actor", "action", "target", "detail"})
	_ = s.store.StreamActivity(r.Context(), func(a *model.Activity) error {
		return cw.Write([]string{a.CreatedAt.Format(time.RFC3339), a.Actor, a.Action, a.Target, a.Detail})
	})
	cw.Flush()
}

// handleExportRuns streams run history as CSV (admin). Optional ?projectId scope.
func (s *Server) handleExportRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="runs-`+time.Now().Format("20060102")+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"time", "id", "project", "name", "app", "action", "status", "version", "triggeredBy", "exitCode", "startedAt", "finishedAt"})
	_ = s.store.StreamRuns(r.Context(), r.URL.Query().Get("projectId"), func(run *model.Run) error {
		exit := ""
		if run.ExitCode != nil {
			exit = strconv.Itoa(*run.ExitCode)
		}
		return cw.Write([]string{
			run.CreatedAt.Format(time.RFC3339), run.ID, run.ProjectName, run.Name, run.App, run.Action,
			run.Status, run.Version, run.TriggeredBy, exit, fmtTimePtr(run.StartedAt), fmtTimePtr(run.FinishedAt),
		})
	})
	cw.Flush()
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

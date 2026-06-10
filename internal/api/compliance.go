package api

import (
	"net/http"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// complianceReport is a single, auditor-friendly document covering a date range:
// the current access-control matrix plus a summary + detail of everything
// security-relevant that happened (activity, secret access, notifications, runs).
type complianceReport struct {
	GeneratedAt time.Time            `json:"generatedAt"`
	GeneratedBy string               `json:"generatedBy"`
	From        time.Time            `json:"from"`
	To          time.Time            `json:"to"`
	Access      []accessControlRow   `json:"accessControl"`
	Summary     complianceSummary    `json:"summary"`
	ActivityBy  map[string]int       `json:"activityByAction"`
	RunsBy      map[string]int       `json:"runsByStatus"`
	SecretAccess []*model.SecretAccessLog `json:"secretAccess"`
}

type accessControlRow struct {
	Username   string `json:"username"`
	Role       string `json:"role"`
	TwoFactor  bool   `json:"twoFactor"`
	EmailOTP   bool   `json:"emailOtp"`
	CreatedAt  string `json:"createdAt"`
}

type complianceSummary struct {
	Users                  int `json:"users"`
	Admins                 int `json:"admins"`
	ActivityEvents         int `json:"activityEvents"`
	SecretAccessEvents     int `json:"secretAccessEvents"`
	NotificationDeliveries int `json:"notificationDeliveries"`
	NotificationFailures   int `json:"notificationFailures"`
	Runs                   int `json:"runs"`
}

// handleComplianceReport builds a consolidated audit report over ?from..?to
// (RFC3339; default = the last 30 days). Admin-only; downloads as JSON.
func (s *Server) handleComplianceReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ctx := r.Context()
	to := time.Now()
	from := to.AddDate(0, 0, -30)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	inRange := func(t time.Time) bool { return !t.Before(from) && !t.After(to) }

	rep := complianceReport{
		GeneratedAt: time.Now(),
		GeneratedBy: s.actor(ctx),
		From:        from,
		To:          to,
		Access:      []accessControlRow{},
		ActivityBy:  map[string]int{},
		RunsBy:      map[string]int{},
		SecretAccess: []*model.SecretAccessLog{},
	}

	// Access-control matrix (point-in-time): every user, role and MFA posture.
	if users, err := s.store.ListUsers(ctx); err == nil {
		for _, u := range users {
			rep.Access = append(rep.Access, accessControlRow{
				Username: u.Username, Role: u.Role, TwoFactor: u.TwoFactorEnabled,
				EmailOTP: u.EmailOTPEnabled, CreatedAt: u.CreatedAt.Format(time.RFC3339),
			})
			rep.Summary.Users++
			if u.Role == model.RoleAdmin {
				rep.Summary.Admins++
			}
		}
	}

	// Activity counts by action (streamed, ranged).
	_ = s.store.StreamActivity(ctx, func(a *model.Activity) error {
		if inRange(a.CreatedAt) {
			rep.ActivityBy[a.Action]++
			rep.Summary.ActivityEvents++
		}
		return nil
	})

	// Run counts by status (streamed, ranged, all projects).
	_ = s.store.StreamRuns(ctx, "", func(run *model.Run) error {
		if inRange(run.CreatedAt) {
			rep.RunsBy[run.Status]++
			rep.Summary.Runs++
		}
		return nil
	})

	// Secret-access trail (complete within range — the key compliance artifact).
	if logs, err := s.store.ListSecretAccessLogsBetween(ctx, from, to); err == nil {
		rep.SecretAccess = logs
		rep.Summary.SecretAccessEvents = len(logs)
	}

	// Notification deliveries (recent, filtered to range).
	if logs, err := s.store.ListNotificationLogs(ctx, 500); err == nil {
		for _, l := range logs {
			if !inRange(l.CreatedAt) {
				continue
			}
			rep.Summary.NotificationDeliveries++
			if !l.OK {
				rep.Summary.NotificationFailures++
			}
		}
	}

	w.Header().Set("Content-Disposition", `attachment; filename="compliance-`+time.Now().Format("20060102")+`.json"`)
	writeJSON(w, http.StatusOK, rep)
}

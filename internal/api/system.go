package api

import (
	"context"
	"net/http"
	"runtime"
	"sync"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

// ansibleVersion is the runner's `ansible --version` output, cached after the
// first successful fetch (it's static for the life of the runner image).
var (
	ansibleVerMu  sync.Mutex
	ansibleVerVal string
)

func (s *Server) ansibleVersion(ctx context.Context) string {
	ansibleVerMu.Lock()
	defer ansibleVerMu.Unlock()
	if ansibleVerVal == "" {
		if v, err := runnerclient.AnsibleVersion(ctx, s.cfg.RunnerHTTP); err == nil {
			ansibleVerVal = v
		}
	}
	return ansibleVerVal
}

// handleSystemInfo returns a diagnostic snapshot of the deployment — version,
// runtime, enabled auth + notification capabilities, task limits, feature flags
// and the runner fleet — for an admin "System information" screen.
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ctx := r.Context()

	// Which notification channel types have at least one enabled channel.
	configured := map[string]bool{}
	if chans, err := s.store.ListNotificationChannels(ctx); err == nil {
		for _, c := range chans {
			if c.Enabled {
				configured[c.Type] = true
			}
		}
	}
	notifyTypes := []string{
		model.NotifyTelegram, model.NotifySlack, model.NotifyTeams, model.NotifyDiscord,
		model.NotifyRocketchat, model.NotifyGoogleChat, model.NotifyGotify, model.NotifyNtfy,
		model.NotifyPushover, model.NotifyDingtalk, model.NotifyWebhook, model.NotifyEmail,
	}
	notifications := make([]map[string]any, 0, len(notifyTypes))
	for _, t := range notifyTypes {
		notifications = append(notifications, map[string]any{"type": t, "configured": configured[t]})
	}

	online, total := 0, 0
	if runners, err := s.store.ListRunners(ctx); err == nil {
		for _, rn := range runners {
			total++
			if runnerStatus(rn) == "online" {
				online++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"system": map[string]any{
			"version":   appVersion,
			"goVersion": runtime.Version(),
			"platform":  runtime.GOOS + "/" + runtime.GOARCH,
		},
		"ansible": s.ansibleVersion(ctx),
		"database": map[string]any{"dialect": "postgres"},
		"auth": map[string]bool{
			"local":     true,
			"totp":      true, // 2FA is always available
			"ldap":      s.cfg.LDAP.Enabled,
			"oidc":      s.oidc != nil,
			"radius":    s.radiusEnabled(),
			"tacacs":    s.tacacsEnabled(),
			"saml":      s.samlEnabled(),
			"github":    s.github != nil,
			"bitbucket": s.bitbucket != nil,
			"emailOtp":  false,
		},
		"notifications": notifications,
		"cluster": map[string]any{
			"highAvailability": true, // scheduler leader-election + Postgres-externalised dispatch queue
		},
		"runners": map[string]any{
			"remoteRunners": true,
			"total":         total,
			"online":        online,
		},
		"taskSettings": map[string]any{
			"maxDurationSec": s.intSetting(ctx, settingMaxTaskDuration),
		},
		"featureFlags": map[string]bool{
			"nonadminCreateProject": s.boolSetting(ctx, settingNonadminCreateProject),
		},
	})
}

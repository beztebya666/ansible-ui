package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// appVersion is reported via ansibleui_build_info.
const appVersion = "1.0"

// handleMetrics exposes a Prometheus text-format snapshot of control-plane state:
// run counts by status, active/queued/awaiting work, projects/templates, runner
// online state + per-runner load. Optional bearer auth via METRICS_TOKEN; lives
// at /metrics (outside /api, so the session guard doesn't apply — restrict at the
// network/ingress level, or set METRICS_TOKEN).
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MetricsToken != "" && r.Header.Get("Authorization") != "Bearer "+s.cfg.MetricsToken {
		writeErr(w, http.StatusUnauthorized, "metrics token required")
		return
	}
	ctx := r.Context()
	byStatus, _ := s.store.CountRunsByStatus(ctx)
	projects, _ := s.store.ListProjects(ctx)
	templates, _ := s.store.ListTemplates(ctx, "")
	runners, _ := s.store.ListRunners(ctx)

	online, offline := 0, 0
	name := map[string]string{}
	for _, rn := range runners {
		name[rn.ID] = rn.Name
		if runnerStatus(rn) == "online" {
			online++
		} else {
			offline++
		}
	}

	// Queue depth + awaiting + per-runner load all come from the runs table now
	// (HA: there is no in-memory dispatch state).
	queueDepth := byStatus[model.StatusQueued]
	awaiting := byStatus[model.StatusAwaiting]
	load, _ := s.store.CountActiveRunsByRunner(ctx)

	var b strings.Builder
	help(&b, "ansibleui_build_info", "gauge", "Build information.")
	line(&b, "ansibleui_build_info", map[string]string{"version": appVersion}, 1)

	help(&b, "ansibleui_runs", "gauge", "Runs in history, by status.")
	for _, st := range []string{
		model.StatusSuccess, model.StatusFailed, model.StatusCanceled,
		model.StatusRunning, model.StatusPending, model.StatusQueued, model.StatusAwaiting,
	} {
		line(&b, "ansibleui_runs", map[string]string{"status": st}, float64(byStatus[st]))
	}

	help(&b, "ansibleui_runs_active", "gauge", "Runs currently running or pending dispatch.")
	line(&b, "ansibleui_runs_active", nil, float64(byStatus[model.StatusRunning]+byStatus[model.StatusPending]))
	help(&b, "ansibleui_dispatch_queue_depth", "gauge", "Launches waiting for a free runner slot.")
	line(&b, "ansibleui_dispatch_queue_depth", nil, float64(queueDepth))
	help(&b, "ansibleui_runs_awaiting_approval", "gauge", "Runs held for manual approval.")
	line(&b, "ansibleui_runs_awaiting_approval", nil, float64(awaiting))

	help(&b, "ansibleui_projects", "gauge", "Number of projects.")
	line(&b, "ansibleui_projects", nil, float64(len(projects)))
	help(&b, "ansibleui_templates", "gauge", "Number of templates.")
	line(&b, "ansibleui_templates", nil, float64(len(templates)))

	help(&b, "ansibleui_runners", "gauge", "Registered runners by online state.")
	line(&b, "ansibleui_runners", map[string]string{"state": "online"}, float64(online))
	line(&b, "ansibleui_runners", map[string]string{"state": "offline"}, float64(offline))
	help(&b, "ansibleui_runner_load", "gauge", "Jobs currently dispatched to a runner.")
	for id, v := range load {
		line(&b, "ansibleui_runner_load", map[string]string{"runner": name[id], "runner_id": id}, float64(v))
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func help(b *strings.Builder, name, typ, h string) {
	b.WriteString("# HELP " + name + " " + h + "\n# TYPE " + name + " " + typ + "\n")
}

// line writes one Prometheus sample. Label values are escaped per the exposition
// format (\\, \" and \n).
func line(b *strings.Builder, name string, labels map[string]string, v float64) {
	b.WriteString(name)
	if len(labels) > 0 {
		b.WriteByte('{')
		first := true
		for k, val := range labels {
			if !first {
				b.WriteByte(',')
			}
			first = false
			esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(val)
			b.WriteString(k + `="` + esc + `"`)
		}
		b.WriteByte('}')
	}
	b.WriteByte(' ')
	b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
	b.WriteByte('\n')
}

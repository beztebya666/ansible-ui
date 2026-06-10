package api

import (
	"context"
	"fmt"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/notify"
)

const settingLongRunAlert = "tasks.long_run_alert_sec" // 0 = off

// checkLongRunning fires a one-time "long-running task" alert for any run that
// has been running longer than the configured threshold. Called from the leader
// scheduler tick (so it runs once cluster-wide); alerts are de-duped in-memory.
func (s *Server) checkLongRunning(ctx context.Context) {
	sec := s.intSetting(ctx, settingLongRunAlert)
	if sec <= 0 {
		return
	}
	runs, err := s.store.ListLongRunningRuns(ctx, sec)
	if err != nil {
		return
	}
	for _, run := range runs {
		if _, already := s.longRunAlerted.LoadOrStore(run.ID, true); already {
			continue
		}
		s.alertLongRun(ctx, run, sec)
	}
}

func (s *Server) alertLongRun(ctx context.Context, run *model.Run, sec int) {
	chans, err := s.store.ListEnabledNotificationChannels(ctx)
	if err != nil || len(chans) == 0 {
		return
	}
	if run.ProjectName == "" && run.ProjectID != "" {
		if p, perr := s.store.GetProject(ctx, run.ProjectID); perr == nil {
			run.ProjectName = p.Name
		}
	}
	title := "ansible·ui · " + run.Name + " — still running"
	text := fmt.Sprintf("running longer than %ds · project=%s · app=%s", sec, run.ProjectName, run.App)
	vars := map[string]string{
		"run": run.Name, "status": "long-running", "project": run.ProjectName, "app": run.App, "id": run.ID,
	}
	for _, ch := range chans {
		if ch.ProjectID != nil && *ch.ProjectID != "" && *ch.ProjectID != run.ProjectID {
			continue
		}
		if !hasEvent(ch.Events, "long-running") { // explicit opt-in only (it's not a terminal event)
			continue
		}
		if err := notify.Dispatch(ctx, ch, title, text, "", vars); err != nil {
			s.log.Warn("long-run alert failed", "channel", ch.ID, "err", err)
		}
	}
	s.recordActivity(ctx, run.TriggeredBy, "run.longRunning", run.Name, run.App)
}

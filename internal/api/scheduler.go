package api

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

// cronParser accepts standard 5-field cron expressions (minute hour dom month dow).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func parseCron(expr string) (cron.Schedule, error) {
	return cronParser.Parse(expr)
}

// StartScheduler runs the cron evaluation loop on whichever replica holds the
// scheduler leadership lock — so cron schedules / autorun / retention fire once
// across any number of replicas (HA). A standby replica retries every 30s and
// takes over when the leader's lock is released (graceful stop or crash).
func (s *Server) StartScheduler(ctx context.Context) {
	s.log.Info("scheduler starting (leader election)")
	announced := false
	for ctx.Err() == nil {
		lead, err := s.store.TryAcquireLeadership(ctx)
		if err != nil {
			s.log.Warn("scheduler: leadership check failed", "err", err)
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		if lead == nil { // another replica leads — stand by
			if !announced {
				s.log.Info("scheduler: standby (another replica is the leader)")
				announced = true
			}
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		s.log.Info("scheduler: this replica is the leader")
		announced = false
		s.leadAndSchedule(ctx, lead) // runs until ctx is done or leadership is lost
	}
}

// leadAndSchedule runs the evaluation loop while this replica is the leader,
// stepping down (so another replica can take over) if the lock connection dies.
func (s *Server) leadAndSchedule(ctx context.Context, lead *store.Leadership) {
	defer lead.Release()
	s.leader.Store(true)        // reflected in the Cluster dashboard
	defer s.leader.Store(false) // we stepped down / lost the lock
	// Run the dispatch-queue drainer while we hold leadership; it stops when this
	// context is cancelled (we stepped down).
	dctx, dcancel := context.WithCancel(ctx)
	defer dcancel()
	go s.runDrainer(dctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.evaluateSchedules(ctx)
	s.evaluateRetention(ctx) // sweep on becoming leader (no-op unless a policy is set)
	s.gcStaleRepoClones(ctx) // clean orphaned repo checkouts on becoming leader
	tick := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// If we've lost the DB session, the advisory lock is already released —
			// step down so a healthy replica can lead.
			if err := lead.Ping(ctx); err != nil {
				s.log.Warn("scheduler: lost leadership (connection died), standing down", "err", err)
				return
			}
			s.evaluateSchedules(ctx)
			s.checkLongRunning(ctx) // fire long-running-task alerts (no-op unless configured)
			// Poll repos for new commits ~once a minute (a no-op unless a template
			// actually opts into autorun, so it never touches an idle repo).
			if tick++; tick%2 == 0 {
				s.evaluateAutorun(ctx)
				s.syncCachedRepos(ctx) // keep cache-enabled repos warm (due-gated)
			}
			// Inventory monitoring: ping monitored hosts ~every 2 min.
			if tick%4 == 0 {
				s.checkHostMonitors(ctx)
			}
			// Retention sweep + stale-clone GC ~hourly.
			if tick%120 == 0 {
				s.evaluateRetention(ctx)
				s.gcStaleRepoClones(ctx)
			}
		}
	}
}

// runDrainer dispatches queued runs while this replica is the leader: on a
// nudge (a launch / finished run on this replica) and on a short poll (to catch
// runs queued by other replicas). Stops when ctx is cancelled (lost leadership).
func (s *Server) runDrainer(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	s.drainQueue(ctx) // pick up anything already queued (incl. survivors of a restart)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.drainCh:
			s.drainQueue(ctx)
		case <-t.C:
			s.drainQueue(ctx)
		}
	}
}

// sleepCtx sleeps for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// evaluateAutorun syncs repositories that back projects with autorun-on-commit
// templates and fires those templates whenever the repo's commit advances. Free
// when no template opts in (it never touches a repo without an autorun template).
func (s *Server) evaluateAutorun(ctx context.Context) {
	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		return
	}
	byRepo := map[string][]*model.Template{} // repoID → templates to fire on a new commit
	for _, p := range projects {
		if p.RepositoryID == nil || *p.RepositoryID == "" {
			continue
		}
		tpls, err := s.store.ListTemplates(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, t := range tpls {
			if t.AutorunOnCommit {
				byRepo[*p.RepositoryID] = append(byRepo[*p.RepositoryID], t)
			}
		}
	}
	for repoID, tpls := range byRepo {
		repo, err := s.store.GetRepository(ctx, repoID)
		if err != nil {
			continue
		}
		prev := repo.LastCommit
		res, serr := s.syncRepository(ctx, repo) // persists the new LastCommit
		if serr != nil || res.Error != "" || res.Commit == "" {
			continue
		}
		// Skip when unchanged, or on the very first observation (baseline only —
		// don't fire just because autorun was switched on).
		if res.Commit == prev || prev == "" {
			continue
		}
		s.log.Info("autorun: repo advanced", "repo", repo.ID, "from", prev, "to", res.Commit, "templates", len(tpls))
		for _, t := range tpls {
			if _, err := s.runTemplateBy(ctx, t, " (commit)", "autorun", nil); err != nil {
				s.log.Error("autorun: launch failed", "template", t.ID, "err", err)
			}
		}
	}
}

// gcStaleRepoClones removes orphaned repository checkouts under DataDir/repos —
// directories whose repository no longer exists (deleted while a runner was
// offline, a restored DB, an interrupted delete). Leader-only; operates on the
// shared/built-in-runner /data. Repo deletion already cleans the live path; this
// is a safety-net sweep so leaked clones don't accumulate.
func (s *Server) gcStaleRepoClones(ctx context.Context) {
	root := filepath.Join(s.cfg.DataDir, "repos")
	entries, err := os.ReadDir(root)
	if err != nil {
		return // no repos dir yet
	}
	repos, err := s.store.ListRepositories(ctx, "")
	if err != nil {
		return
	}
	live := make(map[string]bool, len(repos))
	for _, r := range repos {
		live[r.ID] = true
	}
	for _, e := range entries {
		if !e.IsDir() || live[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err == nil {
			s.log.Info("gc: removed stale repo clone", "repo", e.Name())
		}
	}
}

// repoCacheSyncInterval is how long a cached repo's mirror stays "warm" before
// the leader background-syncs it again.
const repoCacheSyncInterval = 10 * time.Minute

// syncCachedRepos refreshes the mirror of every cache-enabled repository that is
// due (never synced, or older than repoCacheSyncInterval) — so the file tree /
// inventory detection stays current without waiting for a run. Leader-only.
func (s *Server) syncCachedRepos(ctx context.Context) {
	repos, err := s.store.ListRepositories(ctx, "")
	if err != nil {
		return
	}
	for _, repo := range repos {
		if !repo.CacheEnabled || repo.Status == model.RepoSyncing {
			continue
		}
		if repo.LastSyncedAt != nil && time.Since(*repo.LastSyncedAt) < repoCacheSyncInterval {
			continue // still warm
		}
		if _, serr := s.syncRepository(ctx, repo); serr != nil {
			s.log.Debug("repo cache sync failed", "repo", repo.ID, "err", serr)
		}
	}
}

func (s *Server) evaluateSchedules(ctx context.Context) {
	schedules, err := s.store.ListActiveSchedules(ctx)
	if err != nil {
		s.log.Warn("scheduler: list failed", "err", err)
		return
	}
	now := time.Now()
	for _, sc := range schedules {
		// One-time schedule: fire when due, then retire it.
		if sc.Once {
			if sc.NextRunAt != nil && !sc.NextRunAt.After(now) {
				s.log.Info("scheduler: firing one-time schedule", "schedule", sc.ID)
				s.fireSchedule(ctx, sc)
				fired := now
				_ = s.store.SetScheduleTimes(ctx, sc.ID, &fired, nil)
				_ = s.store.SetScheduleActive(ctx, sc.ID, false)
			}
			continue
		}
		sched, err := parseCron(sc.Cron)
		if err != nil {
			s.log.Warn("scheduler: invalid cron", "schedule", sc.ID, "cron", sc.Cron)
			continue
		}
		if sc.NextRunAt == nil {
			next := sched.Next(now)
			_ = s.store.SetScheduleTimes(ctx, sc.ID, nil, &next)
			continue
		}
		if sc.NextRunAt.After(now) {
			continue
		}
		s.log.Info("scheduler: firing schedule", "schedule", sc.ID, "template", sc.TemplateID)
		s.fireSchedule(ctx, sc)
		next := sched.Next(now)
		fired := now
		_ = s.store.SetScheduleTimes(ctx, sc.ID, &fired, &next)
	}
}

func (s *Server) fireSchedule(ctx context.Context, sc *model.Schedule) {
	if sc.WorkflowID != "" { // workflow schedule
		wf, err := s.store.GetWorkflow(ctx, sc.WorkflowID)
		if err != nil {
			s.log.Warn("scheduler: workflow missing", "schedule", sc.ID, "err", err)
			return
		}
		if _, err := s.startWorkflow(ctx, wf, "scheduler", nil); err != nil {
			s.log.Error("scheduler: workflow launch failed", "schedule", sc.ID, "err", err)
		}
		return
	}
	t, err := s.store.GetTemplate(ctx, sc.TemplateID)
	if err != nil {
		s.log.Warn("scheduler: template missing", "schedule", sc.ID, "err", err)
		return
	}
	if _, err := s.runTemplateBy(ctx, t, " (scheduled)", "scheduler", nil); err != nil {
		s.log.Error("scheduler: launch failed", "schedule", sc.ID, "err", err)
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

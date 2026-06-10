package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

// HA dispatch: there is no in-memory queue or per-replica load counter. A launch
// persists the run as `queued` (or `awaiting`) plus an AES-encrypted launch
// envelope; the single **leader** replica's drainer claims queued runs (atomic
// `queued → pending` in Postgres), reading runner load straight from the runs
// table. Capacity is therefore shared across replicas and queued/awaiting runs
// survive a restart.

// launchEnvelope is everything needed to dispatch a run on any replica. It carries
// decrypted secrets (Spec) + git credentials (RepoSpec), so it is only ever stored
// encrypted (see persistEnvelope) and dropped once the run is dispatched.
type launchEnvelope struct {
	Spec      model.ExecSpec    `json:"spec"`
	RepoSpec  model.GitSyncSpec `json:"repoSpec"`
	GitBacked bool              `json:"gitBacked"`
	Tag       string            `json:"tag"`
}

func (s *Server) persistEnvelope(ctx context.Context, runID string, env launchEnvelope) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	blob, err := s.cipher.Encrypt(raw)
	if err != nil {
		return err
	}
	return s.store.SetRunEnvelope(ctx, runID, blob)
}

func (s *Server) loadEnvelope(ctx context.Context, runID string) (launchEnvelope, error) {
	var env launchEnvelope
	blob, err := s.store.GetRunEnvelope(ctx, runID)
	if err != nil {
		return env, err
	}
	if len(blob) == 0 {
		return env, fmt.Errorf("no launch envelope")
	}
	raw, derr := s.cipher.Decrypt(blob)
	if derr != nil {
		return env, derr
	}
	return env, json.Unmarshal(raw, &env)
}

// enqueueRun persists a fresh run as queued with its launch envelope, then nudges
// the drainer. The leader dispatches it now (slot free) or when one frees up.
func (s *Server) enqueueRun(ctx context.Context, run *model.Run, env launchEnvelope) error {
	run.Status = model.StatusQueued
	run.RunnerID = ""
	if err := s.store.CreateRun(ctx, run); err != nil {
		return err
	}
	if err := s.persistEnvelope(ctx, run.ID, env); err != nil {
		return err
	}
	s.recordActivity(ctx, run.TriggeredBy, "run.launched", run.Name, run.App)
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	s.nudgeDrainer()
	return nil
}

// holdForApproval persists a run as awaiting (with its envelope) until a project
// admin approves it; the envelope survives restarts and lets any replica's leader
// dispatch it on approval.
func (s *Server) holdForApproval(ctx context.Context, run *model.Run, env launchEnvelope) error {
	run.Status = model.StatusAwaiting
	run.RunnerID = ""
	if err := s.store.CreateRun(ctx, run); err != nil {
		return err
	}
	if err := s.persistEnvelope(ctx, run.ID, env); err != nil {
		return err
	}
	s.recordActivity(ctx, run.TriggeredBy, "run.awaiting", run.Name, run.App)
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	return nil
}

// dispatchRun materialises the workspace on a remote runner (git-backed) then
// starts streaming the run. Shared by the queue drainer; drops the envelope once
// the run is in flight.
func (s *Server) dispatchRun(run *model.Run, env launchEnvelope, url, id string) error {
	run.RunnerID = id
	_ = s.store.SetRunRunner(context.Background(), run.ID, id)
	_ = s.store.SetRunEnvelope(context.Background(), run.ID, nil)
	if env.GitBacked && url != s.cfg.RunnerURL {
		dctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		rres, rerr := runnerclient.SyncGit(dctx, runnerHTTPBase(url), env.RepoSpec)
		if rerr != nil {
			return errNotReady("workspace sync on runner failed: " + rerr.Error())
		}
		if rres.Error != "" {
			return errNotReady("workspace sync on runner failed: " + rres.Error)
		}
	}
	s.manager.Start(run, env.Spec, url)
	return nil
}

// nudgeDrainer asks the (leader's) drainer to run a pass — non-blocking. On the
// replica that is the leader this dispatches near-instantly; on others it's a
// no-op (the leader's periodic poll catches the new work).
func (s *Server) nudgeDrainer() {
	select {
	case s.drainCh <- struct{}{}:
	default:
	}
}

// onRunFinished is invoked by the manager when a run reaches a terminal state. The
// freed slot is reflected immediately in the runs table (status is already
// terminal); just nudge the drainer to fill it.
func (s *Server) onRunFinished(runnerID string) { s.nudgeDrainer() }

// drainQueue is the leader's dispatch pass: claim queued runs onto runners with
// free capacity (load read from the runs table), oldest first. Only the leader
// runs this, so the in-cycle load map + atomic claim avoid any double-dispatch.
func (s *Server) drainQueue(ctx context.Context) {
	queued, err := s.store.ListQueuedRuns(ctx)
	if err != nil || len(queued) == 0 {
		return
	}
	load, err := s.store.CountActiveRunsByRunner(ctx)
	if err != nil {
		return
	}
	for _, run := range queued {
		env, eerr := s.loadEnvelope(ctx, run.ID)
		if eerr != nil {
			s.log.Warn("queued run has no usable launch envelope — failing", "run", run.ID, "err", eerr)
			s.failRun(run, "launch context was lost (the server restarted before dispatch)")
			continue
		}
		url, id, queue, perr := s.pickRunner(ctx, env.Tag, load)
		if perr != nil || queue {
			continue // no online runner for the tag, or the pool is full → stay queued
		}
		ok, cerr := s.store.ClaimQueuedRun(ctx, run.ID, id)
		if cerr != nil || !ok {
			continue // someone else (or a cancel) took it
		}
		if id != "" {
			load[id]++ // reserve the slot for the rest of this pass
		}
		run.RunnerID = id
		if derr := s.dispatchRun(run, env, url, id); derr != nil {
			if id != "" {
				load[id]--
			}
			s.failRun(run, derr.Error())
		}
	}
}

// pickRunner chooses a runner for a tag honouring per-runner concurrency, using
// the supplied DB-derived load map. queue=true when every matching runner is at
// capacity; err when no runner is online for the tag.
func (s *Server) pickRunner(ctx context.Context, tag string, load map[string]int) (url, id string, queue bool, err error) {
	if tag == "" || tag == "default" {
		if rn, e := s.store.FindOnlineRunnerByTag(ctx, "default"); e == nil {
			return rn.URL, rn.ID, false, nil // built-in default pool is unlimited
		}
		return s.cfg.RunnerURL, "", false, nil
	}
	cands, e := s.store.ListOnlineRunnersByTag(ctx, tag)
	if e != nil {
		return "", "", false, e
	}
	if len(cands) == 0 {
		return "", "", false, fmt.Errorf("no online runner available for tag %q", tag)
	}
	var best *model.Runner
	bestLoad := 0
	for _, rn := range cands {
		l := load[rn.ID]
		if rn.MaxConcurrent > 0 && l >= rn.MaxConcurrent {
			continue
		}
		if best == nil || l < bestLoad {
			best, bestLoad = rn, l
		}
	}
	if best == nil {
		return "", "", true, nil // whole pool busy → queue
	}
	return best.URL, best.ID, false, nil
}

// cancelQueued cancels a still-queued run (atomic queued → canceled in Postgres),
// returning true if it had not yet started.
func (s *Server) cancelQueued(id string) bool {
	ctx := context.Background()
	run, err := s.store.GetRun(ctx, id)
	if err != nil || run.Status != model.StatusQueued {
		return false
	}
	ok, cerr := s.store.CancelQueuedRun(ctx, id)
	if cerr != nil || !ok {
		return false
	}
	_ = s.store.SetRunEnvelope(ctx, id, nil)
	run.Status = model.StatusCanceled
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	s.manager.recordTerminal(ctx, run, model.StatusCanceled)
	return true
}

// failRun marks a run failed with msg as its output (e.g. a dispatch/sync error).
func (s *Server) failRun(run *model.Run, msg string) {
	ctx := context.Background()
	_ = s.store.SetRunEnvelope(ctx, run.ID, nil)
	_ = s.store.FinishRun(ctx, run.ID, model.StatusFailed, -1, model.RunStats{}, []byte(msg+"\r\n"), nil)
	run.Status = model.StatusFailed
	s.events.Publish(map[string]any{"type": "run.updated", "run": run})
	s.manager.recordTerminal(ctx, run, model.StatusFailed)
}

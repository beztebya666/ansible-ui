package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// RunFilter narrows a run listing.
type RunFilter struct {
	ProjectID string
	Status    string
	Limit     int
}

const runSelect = `
	SELECT r.id, r.project_id, r.template_id, r.environment_id, r.name, r.triggered_by, r.app, r.action, r.commit_hash,
	       r.version, r.runner_id, r.playbook, r.status, r.exit_code,
	       r.limit_pattern, r.tags, r.skip_tags, r.extra_vars, r.check_mode, r.diff_mode,
	       r.verbosity, r.cli_args, r.workspace, r.auto_approve,
	       r.args, r.stats, r.artifact_paths, r.workflow_run_id, r.workflow_step,
	       r.created_at, r.started_at, r.finished_at, p.name
	FROM runs r JOIN projects p ON p.id = r.project_id`

// CreateRun inserts a pending run.
func (s *Store) CreateRun(ctx context.Context, r *model.Run) error {
	if r.ID == "" {
		r.ID = NewID("run")
	}
	if r.Status == "" {
		r.Status = model.StatusPending
	}
	if r.App == "" {
		r.App = model.AppAnsible
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO runs (id, project_id, template_id, environment_id, name, triggered_by, app, action, commit_hash,
		    version, runner_id, playbook, status, limit_pattern, tags, skip_tags, extra_vars, check_mode, diff_mode,
		    verbosity, cli_args, workspace, auto_approve, args, stats, secret_vars, artifact_paths,
		    workflow_run_id, workflow_step)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)
		RETURNING created_at`,
		r.ID, r.ProjectID, r.TemplateID, r.EnvironmentID, r.Name, r.TriggeredBy, r.App, r.Action, r.Commit,
		r.Version, r.RunnerID, r.Playbook, r.Status, r.Limit, r.Tags, r.SkipTags, marshalJSON(r.ExtraVars, "{}"),
		r.Check, r.Diff, r.Verbosity, marshalJSON(r.CliArgs, "[]"), r.Workspace, r.AutoApprove,
		marshalJSON(r.Args, "[]"), marshalJSON(r.Stats, "{}"), r.SecretVarsBlob, marshalJSON(r.ArtifactPaths, "[]"),
		r.WorkflowRunID, r.WorkflowStep).
		Scan(&r.CreatedAt)
}

// NextBuildNumber returns the next build number for a build template (1-based,
// monotonic by run count). Assigned at launch so each build run gets an artifact.
func (s *Store) NextBuildNumber(ctx context.Context, templateID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE template_id=$1`, templateID).Scan(&n)
	return n + 1, err
}

// LatestBuildVersions returns, per template, the version of that template's most
// recent successful run (its latest artifact). projectID scopes it (empty = all).
// One query — used to hydrate the templates list with a Semaphore-style VERSION.
func (s *Store) LatestBuildVersions(ctx context.Context, projectID string) (map[string]string, error) {
	q := `SELECT DISTINCT ON (template_id) template_id, version
	      FROM runs
	      WHERE status='success' AND version <> '' AND template_id IS NOT NULL`
	args := []any{}
	if projectID != "" {
		q += ` AND project_id=$1`
		args = append(args, projectID)
	}
	q += ` ORDER BY template_id, created_at DESC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var tid, ver string
		if err := rows.Scan(&tid, &ver); err != nil {
			return nil, err
		}
		out[tid] = ver
	}
	return out, rows.Err()
}

// LatestSuccessfulBuild returns the most recent successful run of a build
// template (the artifact a deploy consumes), or ErrNotFound if there is none.
func (s *Store) LatestSuccessfulBuild(ctx context.Context, templateID string) (*model.Run, error) {
	return scanRun(s.pool.QueryRow(ctx,
		runSelect+` WHERE r.template_id=$1 AND r.status='success' ORDER BY r.created_at DESC LIMIT 1`, templateID))
}

// PrevTerminalStatus returns the status of the most recent terminal (success/
// failed/canceled) run of a template, excluding excludeRunID — used to detect a
// "recovery" (success after a prior failure) for fixed-notifications. "" if none.
func (s *Store) PrevTerminalStatus(ctx context.Context, templateID, excludeRunID string) (string, error) {
	var st string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM runs
		WHERE template_id=$1 AND id<>$2 AND status IN ('success','failed','canceled')
		ORDER BY created_at DESC LIMIT 1`, templateID, excludeRunID).Scan(&st)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return st, err
}

// RequestCancel flags a run for cancellation. Used for cross-replica cancel: a
// replica that isn't streaming the run sets the flag; the owning replica polls
// CancelRequested and cancels the live process.
func (s *Store) RequestCancel(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE runs SET cancel_requested=true WHERE id=$1`, id)
	return err
}

// CancelRequested reports whether a run has been flagged for cancellation.
func (s *Store) CancelRequested(ctx context.Context, id string) (bool, error) {
	var v bool
	err := s.pool.QueryRow(ctx, `SELECT cancel_requested FROM runs WHERE id=$1`, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return v, err
}

// GetRunSecretVars returns the run's encrypted secret-extra-vars blob (may be
// empty). The caller decrypts it — used by the admin-only reveal endpoint.
func (s *Store) GetRunSecretVars(ctx context.Context, id string) ([]byte, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `SELECT secret_vars FROM runs WHERE id=$1`, id).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return blob, err
}

// SetRunStatus updates a run's status (used by the approval flow: awaiting →
// queued/canceled without recreating the row).
func (s *Store) SetRunStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE runs SET status=$2 WHERE id=$1`, id, status)
	return err
}

// SetRunRunner records which runner a run was dispatched to (used when a queued
// run is later assigned a runner as a slot frees up).
func (s *Store) SetRunRunner(ctx context.Context, id, runnerID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE runs SET runner_id=$2 WHERE id=$1`, id, runnerID)
	return err
}

// MarkRunning flips a run to running and stamps started_at.
func (s *Store) MarkRunning(ctx context.Context, id string, args []string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runs SET status=$2, started_at=now(), args=$3 WHERE id=$1`,
		id, model.StatusRunning, marshalJSON(args, "[]"))
	return err
}

// FinishRun records the terminal state, stats, full captured output and the
// per-line completion timestamps (ms) used by the structured log view.
func (s *Store) FinishRun(ctx context.Context, id, status string, exitCode int, stats model.RunStats, output []byte, lineTimes []int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runs SET status=$2, exit_code=$3, stats=$4, output=$5, log_times=$6, finished_at=now()
		WHERE id=$1`,
		id, status, exitCode, marshalJSON(stats, "{}"), output, marshalJSON(lineTimes, "[]"))
	return err
}

// UpdateRunLog persists the in-progress output + per-line timestamps so the live
// structured log view can render a still-running run (status is left untouched).
func (s *Store) UpdateRunLog(ctx context.Context, id string, output []byte, lineTimes []int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE runs SET output=$2, log_times=$3 WHERE id=$1`,
		id, output, marshalJSON(lineTimes, "[]"))
	return err
}

// GetRunLog returns the stored output plus per-line completion timestamps (ms).
func (s *Store) GetRunLog(ctx context.Context, id string) ([]byte, []int64, error) {
	var out, timesJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT output, log_times FROM runs WHERE id=$1`, id).Scan(&out, &timesJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	var times []int64
	_ = json.Unmarshal(timesJSON, &times)
	return out, times, nil
}

// GetRun returns a run (without the heavy output blob).
func (s *Store) GetRun(ctx context.Context, id string) (*model.Run, error) {
	return scanRun(s.pool.QueryRow(ctx, runSelect+` WHERE r.id=$1`, id))
}

// GetRunOutput returns the stored terminal output for replay.
func (s *Store) GetRunOutput(ctx context.Context, id string) ([]byte, error) {
	var out []byte
	err := s.pool.QueryRow(ctx, `SELECT output FROM runs WHERE id=$1`, id).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

// ListRuns returns runs matching the filter, newest first.
func (s *Store) ListRuns(ctx context.Context, f RunFilter) ([]*model.Run, error) {
	q := runSelect
	args := []any{}
	where := ""
	add := func(clause string, val any) {
		args = append(args, val)
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += clause + "$" + itoa(len(args))
	}
	if f.ProjectID != "" {
		add("r.project_id=", f.ProjectID)
	}
	if f.Status != "" {
		add("r.status=", f.Status)
	}
	q += where + " ORDER BY r.created_at DESC"
	if f.Limit > 0 {
		args = append(args, f.Limit)
		q += " LIMIT $" + itoa(len(args))
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountRunsByStatus returns a status → count map across all runs.
func (s *Store) CountRunsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM runs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// MarkOrphansFailed flips any runs still "running" at boot to failed (the
// process they belonged to is gone after a restart).
func (s *Store) MarkOrphansFailed(ctx context.Context) error {
	// Only runs that were actually executing (running/pending) are orphaned — the
	// in-memory streaming state is gone. Queued/awaiting runs carry a persisted
	// launch_envelope, so they survive the restart and the leader re-dispatches /
	// re-holds them; a queued run with no envelope (legacy) is failed by the drainer.
	_, err := s.pool.Exec(ctx, `
		UPDATE runs SET status=$1, finished_at=now()
		WHERE status IN ($2,$3)`,
		model.StatusFailed, model.StatusRunning, model.StatusPending)
	return err
}

// SetRunEnvelope stores the AES-encrypted launch envelope for a queued/awaiting run.
func (s *Store) SetRunEnvelope(ctx context.Context, id string, blob []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE runs SET launch_envelope=$2 WHERE id=$1`, id, blob)
	return err
}

// GetRunEnvelope returns the encrypted launch envelope (nil if none).
func (s *Store) GetRunEnvelope(ctx context.Context, id string) ([]byte, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `SELECT launch_envelope FROM runs WHERE id=$1`, id).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return blob, err
}

// ClaimQueuedRun atomically transitions a run queued → pending and assigns a
// runner. Returns true only for the caller that won the row, so the (leader)
// drainer never double-dispatches even across replicas.
func (s *Store) ClaimQueuedRun(ctx context.Context, id, runnerID string) (bool, error) {
	ct, err := s.pool.Exec(ctx,
		`UPDATE runs SET status=$2, runner_id=$3 WHERE id=$1 AND status=$4`,
		id, model.StatusPending, runnerID, model.StatusQueued)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}

// CancelQueuedRun atomically transitions a run queued → canceled (only succeeds
// if it had not yet been claimed for dispatch).
func (s *Store) CancelQueuedRun(ctx context.Context, id string) (bool, error) {
	ct, err := s.pool.Exec(ctx, `
		UPDATE runs SET status=$2, finished_at=now() WHERE id=$1 AND status=$3`,
		id, model.StatusCanceled, model.StatusQueued)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}

// ListQueuedRuns returns queued runs, oldest first (FIFO dispatch order).
func (s *Store) ListQueuedRuns(ctx context.Context) ([]*model.Run, error) {
	rows, err := s.pool.Query(ctx, runSelect+` WHERE r.status='queued' ORDER BY r.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountActiveRunsByRunner returns, per runner id, how many runs are currently
// occupying a slot (pending or running). This is the shared, cross-replica
// runner load — no in-memory counter needed.
func (s *Store) CountActiveRunsByRunner(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT coalesce(runner_id,''), count(*) FROM runs
		WHERE status IN ('pending','running') GROUP BY runner_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ListLongRunningRuns returns runs that have been running longer than thresholdSec
// — used to fire "long-running task" alerts.
func (s *Store) ListLongRunningRuns(ctx context.Context, thresholdSec int) ([]*model.Run, error) {
	rows, err := s.pool.Query(ctx,
		runSelect+` WHERE r.status='running' AND r.started_at IS NOT NULL AND r.started_at < now() - make_interval(secs => $1)`, thresholdSec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountActiveRunsByTemplate counts a template's runs currently occupying a slot
// (pending or running) — used to enforce a per-template concurrency cap.
func (s *Store) CountActiveRunsByTemplate(ctx context.Context, templateID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM runs WHERE template_id=$1 AND status IN ('queued','pending','running')`, templateID).Scan(&n)
	return n, err
}

// CountActiveRuns counts every queued/pending/running run across all projects —
// for the global max-parallel cap.
func (s *Store) CountActiveRuns(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM runs WHERE status IN ('queued','pending','running')`).Scan(&n)
	return n, err
}

// DeleteRun removes a single run. Returns ErrNotFound if it doesn't exist.
func (s *Store) DeleteRun(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM runs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearRuns deletes finished runs (optionally scoped to a project), leaving any
// active/pending run intact. Returns the removed run IDs so the caller can drop
// their on-disk artifacts.
func (s *Store) ClearRuns(ctx context.Context, projectID string) ([]string, error) {
	q := `DELETE FROM runs WHERE status NOT IN ($1,$2,$3,$4)`
	args := []any{model.StatusRunning, model.StatusPending, model.StatusQueued, model.StatusAwaiting}
	if projectID != "" {
		q += ` AND project_id=$5`
		args = append(args, projectID)
	}
	q += ` RETURNING id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// StreamRuns calls fn for every run (newest first), optionally scoped to a
// project — used by the CSV export.
func (s *Store) StreamRuns(ctx context.Context, projectID string, fn func(*model.Run) error) error {
	q := runSelect
	args := []any{}
	if projectID != "" {
		q += ` WHERE r.project_id=$1`
		args = append(args, projectID)
	}
	q += ` ORDER BY r.created_at DESC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return err
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

// DeleteRunsOlderThan removes finished runs created before now-days and returns
// their IDs (so the caller can drop on-disk artifacts). Active/queued runs are
// always preserved. days must be > 0.
func (s *Store) DeleteRunsOlderThan(ctx context.Context, days int) ([]string, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	rows, err := s.pool.Query(ctx, `
		DELETE FROM runs
		WHERE status NOT IN ($1,$2,$3,$4) AND created_at < $5
		RETURNING id`,
		model.StatusRunning, model.StatusPending, model.StatusQueued, model.StatusAwaiting, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanRun(row rowScanner) (*model.Run, error) {
	var r model.Run
	var ev, args, stats, cliArgs, artifactPaths []byte
	err := row.Scan(&r.ID, &r.ProjectID, &r.TemplateID, &r.EnvironmentID, &r.Name, &r.TriggeredBy, &r.App, &r.Action, &r.Commit,
		&r.Version, &r.RunnerID, &r.Playbook, &r.Status,
		&r.ExitCode, &r.Limit, &r.Tags, &r.SkipTags, &ev, &r.Check, &r.Diff, &r.Verbosity,
		&cliArgs, &r.Workspace, &r.AutoApprove,
		&args, &stats, &artifactPaths, &r.WorkflowRunID, &r.WorkflowStep,
		&r.CreatedAt, &r.StartedAt, &r.FinishedAt, &r.ProjectName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ExtraVars = unmarshalMap(ev)
	_ = json.Unmarshal(args, &r.Args)
	_ = json.Unmarshal(stats, &r.Stats)
	_ = json.Unmarshal(cliArgs, &r.CliArgs)
	_ = json.Unmarshal(artifactPaths, &r.ArtifactPaths)
	return &r, nil
}

// itoa avoids strconv churn in query building.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

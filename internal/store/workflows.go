package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// ---- workflows ---------------------------------------------------------

func (s *Store) CreateWorkflow(ctx context.Context, w *model.Workflow) error {
	if w.ID == "" {
		w.ID = NewID("wf")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO workflows (id, project_id, name, description, steps, variables)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at, updated_at`,
		w.ID, w.ProjectID, w.Name, w.Description, marshalJSON(w.Steps, "[]"), marshalJSON(w.Variables, "{}")).
		Scan(&w.CreatedAt, &w.UpdatedAt)
}

func (s *Store) UpdateWorkflow(ctx context.Context, w *model.Workflow) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE workflows SET name=$2, description=$3, steps=$4, variables=$5, updated_at=now() WHERE id=$1`,
		w.ID, w.Name, w.Description, marshalJSON(w.Steps, "[]"), marshalJSON(w.Variables, "{}"))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetWorkflow(ctx context.Context, id string) (*model.Workflow, error) {
	return scanWorkflow(s.pool.QueryRow(ctx,
		`SELECT id, project_id, name, description, steps, variables, created_at, updated_at FROM workflows WHERE id=$1`, id))
}

func (s *Store) ListWorkflows(ctx context.Context, projectID string) ([]*model.Workflow, error) {
	q := `SELECT id, project_id, name, description, steps, variables, created_at, updated_at FROM workflows`
	args := []any{}
	if projectID != "" {
		q += ` WHERE project_id=$1`
		args = append(args, projectID)
	}
	q += ` ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWorkflow(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM workflows WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanWorkflow(row rowScanner) (*model.Workflow, error) {
	var w model.Workflow
	var steps, vars []byte
	err := row.Scan(&w.ID, &w.ProjectID, &w.Name, &w.Description, &steps, &vars, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(steps, &w.Steps)
	_ = json.Unmarshal(vars, &w.Variables)
	return &w, nil
}

// ---- workflow versions -------------------------------------------------

// SnapshotWorkflowVersion records the workflow's current definition as the next
// version number (max+1). Best-effort — a failure must not block the edit.
func (s *Store) SnapshotWorkflowVersion(ctx context.Context, w *model.Workflow, actor string) error {
	var next int
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM workflow_versions WHERE workflow_id=$1`, w.ID).Scan(&next)
	if next < 1 {
		next = 1
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO workflow_versions (id, workflow_id, version, name, description, steps, variables, actor)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		NewID("wfv"), w.ID, next, w.Name, w.Description,
		marshalJSON(w.Steps, "[]"), marshalJSON(w.Variables, "{}"), actor)
	return err
}

// ListWorkflowVersions returns a workflow's version snapshots, newest first.
func (s *Store) ListWorkflowVersions(ctx context.Context, workflowID string) ([]*model.WorkflowVersion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workflow_id, version, name, description, steps, variables, actor, created_at
		FROM workflow_versions WHERE workflow_id=$1 ORDER BY version DESC`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.WorkflowVersion{}
	for rows.Next() {
		v, err := scanWorkflowVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetWorkflowVersion returns one version snapshot.
func (s *Store) GetWorkflowVersion(ctx context.Context, id string) (*model.WorkflowVersion, error) {
	return scanWorkflowVersion(s.pool.QueryRow(ctx, `
		SELECT id, workflow_id, version, name, description, steps, variables, actor, created_at
		FROM workflow_versions WHERE id=$1`, id))
}

func scanWorkflowVersion(row rowScanner) (*model.WorkflowVersion, error) {
	var v model.WorkflowVersion
	var steps, vars []byte
	err := row.Scan(&v.ID, &v.WorkflowID, &v.Version, &v.Name, &v.Description, &steps, &vars, &v.Actor, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(steps, &v.Steps)
	_ = json.Unmarshal(vars, &v.Variables)
	return &v, nil
}

// ---- workflow runs -----------------------------------------------------

const wfRunSelect = `
	SELECT wr.id, wr.workflow_id, wr.project_id, wr.name, wr.triggered_by, wr.status, wr.steps, wr.variables,
	       wr.created_at, wr.started_at, wr.finished_at, w.name, p.name
	FROM workflow_runs wr
	JOIN workflows w ON w.id = wr.workflow_id
	JOIN projects p ON p.id = wr.project_id`

func (s *Store) CreateWorkflowRun(ctx context.Context, wr *model.WorkflowRun) error {
	if wr.ID == "" {
		wr.ID = NewID("wfr")
	}
	if wr.Status == "" {
		wr.Status = model.WFStatusRunning
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO workflow_runs (id, workflow_id, project_id, name, triggered_by, status, steps, variables, started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now()) RETURNING created_at, started_at`,
		wr.ID, wr.WorkflowID, wr.ProjectID, wr.Name, wr.TriggeredBy, wr.Status, marshalJSON(wr.Steps, "[]"), marshalJSON(wr.Variables, "{}")).
		Scan(&wr.CreatedAt, &wr.StartedAt)
}

// UpdateWorkflowRun persists the step states + status; sets finished_at on a
// terminal status.
func (s *Store) UpdateWorkflowRun(ctx context.Context, wr *model.WorkflowRun) error {
	terminal := wr.Status != model.WFStatusRunning
	_, err := s.pool.Exec(ctx, `
		UPDATE workflow_runs SET status=$2, steps=$3, variables=$5,
		    finished_at = CASE WHEN $4 AND finished_at IS NULL THEN now() ELSE finished_at END
		WHERE id=$1`,
		wr.ID, wr.Status, marshalJSON(wr.Steps, "[]"), terminal, marshalJSON(wr.Variables, "{}"))
	return err
}

func (s *Store) GetWorkflowRun(ctx context.Context, id string) (*model.WorkflowRun, error) {
	return scanWorkflowRun(s.pool.QueryRow(ctx, wfRunSelect+` WHERE wr.id=$1`, id))
}

// ListWorkflowRuns returns recent workflow runs, optionally scoped to a project
// and/or a workflow.
func (s *Store) ListWorkflowRuns(ctx context.Context, projectID, workflowID string, limit int) ([]*model.WorkflowRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := wfRunSelect
	args := []any{}
	where := ""
	add := func(clause string, v any) {
		args = append(args, v)
		if where == "" {
			where = " WHERE "
		} else {
			where += " AND "
		}
		where += clause + "$" + itoa(len(args))
	}
	if projectID != "" {
		add("wr.project_id=", projectID)
	}
	if workflowID != "" {
		add("wr.workflow_id=", workflowID)
	}
	args = append(args, limit)
	q += where + ` ORDER BY wr.created_at DESC LIMIT $` + itoa(len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.WorkflowRun
	for rows.Next() {
		wr, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wr)
	}
	return out, rows.Err()
}

func scanWorkflowRun(row rowScanner) (*model.WorkflowRun, error) {
	var wr model.WorkflowRun
	var steps, vars []byte
	err := row.Scan(&wr.ID, &wr.WorkflowID, &wr.ProjectID, &wr.Name, &wr.TriggeredBy, &wr.Status, &steps, &vars,
		&wr.CreatedAt, &wr.StartedAt, &wr.FinishedAt, &wr.WorkflowName, &wr.ProjectName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(steps, &wr.Steps)
	_ = json.Unmarshal(vars, &wr.Variables)
	return &wr, nil
}

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const scheduleSelect = `
	SELECT s.id, s.template_id, s.workflow_id, s.name, s.cron, s.once, s.active, s.last_run_at, s.next_run_at,
	       s.created_at, s.updated_at, COALESCE(t.name, w.name, ''), COALESCE(t.project_id, w.project_id, '')
	FROM schedules s
	LEFT JOIN templates t ON t.id = s.template_id
	LEFT JOIN workflows w ON w.id = s.workflow_id`

// CreateSchedule inserts a schedule (targets a template OR a workflow).
func (s *Store) CreateSchedule(ctx context.Context, sc *model.Schedule) error {
	if sc.ID == "" {
		sc.ID = NewID("sch")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO schedules (id, template_id, workflow_id, name, cron, once, active, next_run_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at, updated_at`,
		sc.ID, nilIfEmpty(sc.TemplateID), nilIfEmpty(sc.WorkflowID), sc.Name, sc.Cron, sc.Once, sc.Active, sc.NextRunAt).
		Scan(&sc.CreatedAt, &sc.UpdatedAt)
}

// nilIfEmpty returns nil for "" so a nullable FK column stores NULL (not "").
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UpdateSchedule updates editable fields.
func (s *Store) UpdateSchedule(ctx context.Context, sc *model.Schedule) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE schedules SET name=$2, cron=$3, once=$4, active=$5, next_run_at=$6, updated_at=now()
		WHERE id=$1`,
		sc.ID, sc.Name, sc.Cron, sc.Once, sc.Active, sc.NextRunAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetScheduleActive flips a schedule's active flag (used to retire one-time runs).
func (s *Store) SetScheduleActive(ctx context.Context, id string, active bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE schedules SET active=$2, updated_at=now() WHERE id=$1`, id, active)
	return err
}

// SetScheduleTimes records a fire (last + next).
func (s *Store) SetScheduleTimes(ctx context.Context, id string, last, next *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE schedules SET last_run_at=COALESCE($2, last_run_at), next_run_at=$3 WHERE id=$1`,
		id, last, next)
	return err
}

// GetSchedule returns one schedule.
func (s *Store) GetSchedule(ctx context.Context, id string) (*model.Schedule, error) {
	return scanSchedule(s.pool.QueryRow(ctx, scheduleSelect+` WHERE s.id=$1`, id))
}

// ListSchedules returns all schedules (optionally by template).
func (s *Store) ListSchedules(ctx context.Context, templateID string) ([]*model.Schedule, error) {
	q := scheduleSelect + ` ORDER BY s.created_at`
	args := []any{}
	if templateID != "" {
		q = scheduleSelect + ` WHERE s.template_id=$1 ORDER BY s.created_at`
		args = append(args, templateID)
	}
	return s.querySchedules(ctx, q, args...)
}

// ListActiveSchedules returns active schedules (for the scheduler loop).
func (s *Store) ListActiveSchedules(ctx context.Context) ([]*model.Schedule, error) {
	return s.querySchedules(ctx, scheduleSelect+` WHERE s.active = true`)
}

func (s *Store) querySchedules(ctx context.Context, q string, args ...any) ([]*model.Schedule, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// DeleteSchedule removes a schedule.
func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM schedules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSchedule(row rowScanner) (*model.Schedule, error) {
	var sc model.Schedule
	var tid, wid *string
	err := row.Scan(&sc.ID, &tid, &wid, &sc.Name, &sc.Cron, &sc.Once, &sc.Active, &sc.LastRunAt,
		&sc.NextRunAt, &sc.CreatedAt, &sc.UpdatedAt, &sc.TemplateName, &sc.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if tid != nil {
		sc.TemplateID = *tid
	}
	if wid != nil {
		sc.WorkflowID = *wid
	}
	return &sc, err
}

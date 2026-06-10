package store

import (
	"context"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// AddActivity records an audit/activity event (best-effort; errors are ignored
// by callers so logging never breaks the main flow).
func (s *Store) AddActivity(ctx context.Context, a *model.Activity) error {
	if a.ID == "" {
		a.ID = NewID("act")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO activity (id, actor, action, target, detail)
		VALUES ($1,$2,$3,$4,$5) RETURNING created_at`,
		a.ID, a.Actor, a.Action, a.Target, a.Detail).Scan(&a.CreatedAt)
}

// ListActivity returns the most recent activity events.
func (s *Store) ListActivity(ctx context.Context, limit int) ([]*model.Activity, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor, action, target, detail, created_at
		FROM activity ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Activity
	for rows.Next() {
		var a model.Activity
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// StreamActivity calls fn for every audit event (oldest first) — used by the CSV
// export so the whole history can be written without buffering it all in memory.
func (s *Store) StreamActivity(ctx context.Context, fn func(*model.Activity) error) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor, action, target, detail, created_at
		FROM activity ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a model.Activity
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			return err
		}
		if err := fn(&a); err != nil {
			return err
		}
	}
	return rows.Err()
}

// DeleteActivityOlderThan removes audit events created before now-days. days > 0.
func (s *Store) DeleteActivityOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	ct, err := s.pool.Exec(ctx, `DELETE FROM activity WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

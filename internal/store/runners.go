package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const runnerSelect = `
	SELECT id, name, url, tags, platform, version, max_concurrent, builtin, last_seen_at, created_at, updated_at
	FROM runners`

// UpsertRunner registers (or refreshes) a runner keyed by its URL, stamping the
// heartbeat. The built-in runner is seeded with builtin=true.
func (s *Store) UpsertRunner(ctx context.Context, r *model.Runner) error {
	if r.ID == "" {
		r.ID = NewID("rnr")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO runners (id, name, url, tags, platform, version, max_concurrent, builtin, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		ON CONFLICT (url) DO UPDATE SET
		    name=excluded.name, tags=excluded.tags, platform=excluded.platform,
		    version=excluded.version, max_concurrent=excluded.max_concurrent, last_seen_at=now(), updated_at=now()
		RETURNING id, created_at, updated_at`,
		r.ID, r.Name, r.URL, marshalJSON(r.Tags, "[]"), r.Platform, r.Version, r.MaxConcurrent, r.Builtin).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

// HeartbeatRunner stamps a runner's last-seen time.
func (s *Store) HeartbeatRunner(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE runners SET last_seen_at=now() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRunners returns all runners (built-in first, then by name).
func (s *Store) ListRunners(ctx context.Context) ([]*model.Runner, error) {
	rows, err := s.pool.Query(ctx, runnerSelect+` ORDER BY builtin DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Runner
	for rows.Next() {
		r, err := scanRunner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRunner returns one runner by id.
func (s *Store) GetRunner(ctx context.Context, id string) (*model.Runner, error) {
	return scanRunner(s.pool.QueryRow(ctx, runnerSelect+` WHERE id=$1`, id))
}

// FindOnlineRunnerByTag returns an online runner advertising tag (most recently
// seen first), or ErrNotFound when none is available.
func (s *Store) FindOnlineRunnerByTag(ctx context.Context, tag string) (*model.Runner, error) {
	return scanRunner(s.pool.QueryRow(ctx, runnerSelect+`
		WHERE tags @> jsonb_build_array($1::text)
		  AND (builtin OR last_seen_at > now() - interval '45 seconds')
		ORDER BY last_seen_at DESC NULLS LAST LIMIT 1`, tag))
}

// ListOnlineRunnersByTag returns all online runners advertising tag (for
// capacity-aware dispatch across a pool).
func (s *Store) ListOnlineRunnersByTag(ctx context.Context, tag string) ([]*model.Runner, error) {
	rows, err := s.pool.Query(ctx, runnerSelect+`
		WHERE tags @> jsonb_build_array($1::text)
		  AND (builtin OR last_seen_at > now() - interval '45 seconds')
		ORDER BY last_seen_at DESC NULLS LAST`, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Runner
	for rows.Next() {
		r, err := scanRunner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRunner removes a runner from the registry.
func (s *Store) DeleteRunner(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM runners WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRunner(row rowScanner) (*model.Runner, error) {
	var r model.Runner
	var tags []byte
	err := row.Scan(&r.ID, &r.Name, &r.URL, &tags, &r.Platform, &r.Version, &r.MaxConcurrent, &r.Builtin, &r.LastSeenAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(tags, &r.Tags)
	return &r, nil
}

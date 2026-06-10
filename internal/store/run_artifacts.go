package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// CreateRunArtifact records a captured artifact for a run.
func (s *Store) CreateRunArtifact(ctx context.Context, a *model.RunArtifact) error {
	if a.ID == "" {
		a.ID = NewID("art")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO run_artifacts (id, run_id, name, size) VALUES ($1,$2,$3,$4)
		RETURNING created_at`, a.ID, a.RunID, a.Name, a.Size).Scan(&a.CreatedAt)
}

// ListRunArtifacts returns a run's captured artifacts (by name).
func (s *Store) ListRunArtifacts(ctx context.Context, runID string) ([]*model.RunArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, name, size, created_at FROM run_artifacts WHERE run_id=$1 ORDER BY name`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.RunArtifact
	for rows.Next() {
		var a model.RunArtifact
		if err := rows.Scan(&a.ID, &a.RunID, &a.Name, &a.Size, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// GetRunArtifact returns one artifact row.
func (s *Store) GetRunArtifact(ctx context.Context, id string) (*model.RunArtifact, error) {
	var a model.RunArtifact
	err := s.pool.QueryRow(ctx, `
		SELECT id, run_id, name, size, created_at FROM run_artifacts WHERE id=$1`, id).
		Scan(&a.ID, &a.RunID, &a.Name, &a.Size, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

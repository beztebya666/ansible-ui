package store

import (
	"context"
	"encoding/json"

	"github.com/nikiv/ansible-ui/internal/model"
)

// RecordPushedBranch persists a branch the UI committed + pushed.
func (s *Store) RecordPushedBranch(ctx context.Context, b *model.PushedBranch) error {
	if b.ID == "" {
		b.ID = NewID("pb")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO pushed_branches (id, project_id, branch, commit_hash, pr_url, files, actor)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`,
		b.ID, b.ProjectID, b.Branch, b.Commit, b.PRURL, marshalJSON(b.Files, "[]"), b.Actor).
		Scan(&b.CreatedAt)
}

// ListPushedBranches returns a project's pushed branches, newest first.
func (s *Store) ListPushedBranches(ctx context.Context, projectID string) ([]*model.PushedBranch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, branch, commit_hash, pr_url, files, actor, created_at
		FROM pushed_branches WHERE project_id=$1 ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.PushedBranch
	for rows.Next() {
		var b model.PushedBranch
		var files []byte
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Branch, &b.Commit, &b.PRURL, &files, &b.Actor, &b.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(files, &b.Files)
		out = append(out, &b)
	}
	return out, rows.Err()
}

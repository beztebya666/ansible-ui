package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const repoSelect = `
	SELECT id, project_id, name, git_url, branch, credential_id, status, last_commit, last_error,
	       last_synced_at, cache_enabled, created_at, updated_at FROM repositories`

// CreateRepository inserts a repository.
func (s *Store) CreateRepository(ctx context.Context, r *model.Repository) error {
	if r.ID == "" {
		r.ID = NewID("repo")
	}
	if r.Branch == "" {
		r.Branch = "main"
	}
	if r.Status == "" {
		r.Status = model.RepoUnknown
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO repositories (id, project_id, name, git_url, branch, credential_id, status, cache_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at, updated_at`,
		r.ID, r.ProjectID, r.Name, r.GitURL, r.Branch, r.CredentialID, r.Status, r.CacheEnabled).Scan(&r.CreatedAt, &r.UpdatedAt)
}

// UpdateRepository updates editable fields.
func (s *Store) UpdateRepository(ctx context.Context, r *model.Repository) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE repositories SET name=$2, git_url=$3, branch=$4, credential_id=$5, cache_enabled=$6, updated_at=now()
		WHERE id=$1`,
		r.ID, r.Name, r.GitURL, r.Branch, r.CredentialID, r.CacheEnabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRepoStatus records sync progress/results.
func (s *Store) SetRepoStatus(ctx context.Context, id, status, commit, syncErr string, synced bool) error {
	if synced {
		_, err := s.pool.Exec(ctx, `
			UPDATE repositories SET status=$2, last_commit=$3, last_error=$4, last_synced_at=now(), updated_at=now()
			WHERE id=$1`, id, status, commit, syncErr)
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE repositories SET status=$2, last_error=$3, updated_at=now() WHERE id=$1`,
		id, status, syncErr)
	return err
}

// GetRepository returns one repository.
func (s *Store) GetRepository(ctx context.Context, id string) (*model.Repository, error) {
	return scanRepo(s.pool.QueryRow(ctx, repoSelect+` WHERE id=$1`, id))
}

// ListRepositories returns repositories for a project plus shared ones
// (projectID empty = all).
func (s *Store) ListRepositories(ctx context.Context, projectID string) ([]*model.Repository, error) {
	q := repoSelect + ` ORDER BY created_at`
	args := []any{}
	if projectID != "" {
		q = repoSelect + ` WHERE (project_id = $1 OR project_id IS NULL) ORDER BY created_at`
		args = append(args, projectID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Repository
	for rows.Next() {
		r, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRepository removes a repository.
func (s *Store) DeleteRepository(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM repositories WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRepo(row rowScanner) (*model.Repository, error) {
	var r model.Repository
	err := row.Scan(&r.ID, &r.ProjectID, &r.Name, &r.GitURL, &r.Branch, &r.CredentialID, &r.Status,
		&r.LastCommit, &r.LastError, &r.LastSyncedAt, &r.CacheEnabled, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const viewSelect = `SELECT id, project_id, name, app, search, position, created_at, updated_at FROM template_views`

// CreateTemplateView inserts a saved view.
func (s *Store) CreateTemplateView(ctx context.Context, v *model.TemplateView) error {
	if v.ID == "" {
		v.ID = NewID("view")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO template_views (id, project_id, name, app, search, position)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at, updated_at`,
		v.ID, v.ProjectID, v.Name, v.App, v.Search, v.Position).Scan(&v.CreatedAt, &v.UpdatedAt)
}

// UpdateTemplateView updates a saved view.
func (s *Store) UpdateTemplateView(ctx context.Context, v *model.TemplateView) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE template_views SET name=$2, app=$3, search=$4, position=$5, updated_at=now() WHERE id=$1`,
		v.ID, v.Name, v.App, v.Search, v.Position)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTemplateView returns one view.
func (s *Store) GetTemplateView(ctx context.Context, id string) (*model.TemplateView, error) {
	return scanView(s.pool.QueryRow(ctx, viewSelect+` WHERE id=$1`, id))
}

// ListTemplateViews returns views for a project plus shared ones, by position.
func (s *Store) ListTemplateViews(ctx context.Context, projectID string) ([]*model.TemplateView, error) {
	q := viewSelect + ` ORDER BY position, created_at`
	args := []any{}
	if projectID != "" {
		q = viewSelect + ` WHERE (project_id = $1 OR project_id IS NULL) ORDER BY position, created_at`
		args = append(args, projectID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.TemplateView
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteTemplateView removes a view.
func (s *Store) DeleteTemplateView(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM template_views WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanView(row rowScanner) (*model.TemplateView, error) {
	var v model.TemplateView
	err := row.Scan(&v.ID, &v.ProjectID, &v.Name, &v.App, &v.Search, &v.Position, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &v, err
}

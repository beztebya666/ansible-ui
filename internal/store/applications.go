package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const applicationSelect = `SELECT id, name, icon, bin, args, priority, active, kind, created_at, updated_at FROM applications`

// UpsertApplication inserts or updates an application by id.
func (s *Store) UpsertApplication(ctx context.Context, a *model.Application) error {
	if a.Kind == "" {
		a.Kind = model.AppKindCustom
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO applications (id, name, icon, bin, args, priority, active, kind)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET name=excluded.name, icon=excluded.icon, bin=excluded.bin,
		    args=excluded.args, priority=excluded.priority, active=excluded.active, updated_at=now()
		RETURNING created_at, updated_at`,
		a.ID, a.Name, a.Icon, a.Bin, marshalJSON(a.Args, "[]"), a.Priority, a.Active, a.Kind).
		Scan(&a.CreatedAt, &a.UpdatedAt)
}

// CreateApplicationIfMissing seeds a built-in only if it doesn't exist yet.
func (s *Store) CreateApplicationIfMissing(ctx context.Context, a *model.Application) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO applications (id, name, icon, bin, args, priority, active, kind)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO NOTHING`,
		a.ID, a.Name, a.Icon, a.Bin, marshalJSON(a.Args, "[]"), a.Priority, a.Active, a.Kind)
	return err
}

// GetApplication returns one application.
func (s *Store) GetApplication(ctx context.Context, id string) (*model.Application, error) {
	return scanApplication(s.pool.QueryRow(ctx, applicationSelect+` WHERE id=$1`, id))
}

// ListApplications returns all applications ordered by priority.
func (s *Store) ListApplications(ctx context.Context) ([]*model.Application, error) {
	rows, err := s.pool.Query(ctx, applicationSelect+` ORDER BY priority, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Application
	for rows.Next() {
		a, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteApplication removes an application (custom only — enforced in the API).
func (s *Store) DeleteApplication(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM applications WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanApplication(row rowScanner) (*model.Application, error) {
	var a model.Application
	var args []byte
	err := row.Scan(&a.ID, &a.Name, &a.Icon, &a.Bin, &args, &a.Priority, &a.Active, &a.Kind, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(args, &a.Args)
	return &a, nil
}

package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const customRoleSelect = `SELECT id, name, description, permissions, created_at, updated_at FROM custom_roles`

// CreateCustomRole inserts a custom role.
func (s *Store) CreateCustomRole(ctx context.Context, c *model.CustomRole) error {
	if c.ID == "" {
		c.ID = NewID("role")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO custom_roles (id, name, description, permissions)
		VALUES ($1,$2,$3,$4) RETURNING created_at, updated_at`,
		c.ID, c.Name, c.Description, marshalJSON(c.Permissions, "[]")).Scan(&c.CreatedAt, &c.UpdatedAt)
}

// UpdateCustomRole updates name/description/permissions.
func (s *Store) UpdateCustomRole(ctx context.Context, c *model.CustomRole) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE custom_roles SET name=$2, description=$3, permissions=$4, updated_at=now() WHERE id=$1`,
		c.ID, c.Name, c.Description, marshalJSON(c.Permissions, "[]"))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetCustomRole returns one custom role.
func (s *Store) GetCustomRole(ctx context.Context, id string) (*model.CustomRole, error) {
	return scanCustomRole(s.pool.QueryRow(ctx, customRoleSelect+` WHERE id=$1`, id))
}

// ListCustomRoles returns all custom roles, oldest first.
func (s *Store) ListCustomRoles(ctx context.Context) ([]*model.CustomRole, error) {
	rows, err := s.pool.Query(ctx, customRoleSelect+` ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.CustomRole{}
	for rows.Next() {
		c, err := scanCustomRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCustomRole removes a custom role.
func (s *Store) DeleteCustomRole(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM custom_roles WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCustomRole(row rowScanner) (*model.CustomRole, error) {
	var c model.CustomRole
	var perms []byte
	err := row.Scan(&c.ID, &c.Name, &c.Description, &perms, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Permissions = []string{}
	_ = json.Unmarshal(perms, &c.Permissions)
	return &c, nil
}

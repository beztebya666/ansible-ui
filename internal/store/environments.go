package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const environmentSelect = `
	SELECT id, project_id, name, description, extra_vars, env_vars, secrets, external_secrets, created_at, updated_at FROM environments`

// CreateEnvironment inserts an environment.
func (s *Store) CreateEnvironment(ctx context.Context, e *model.Environment) error {
	if e.ID == "" {
		e.ID = NewID("env")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO environments (id, project_id, name, description, extra_vars, env_vars, secrets, external_secrets)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at, updated_at`,
		e.ID, e.ProjectID, e.Name, e.Description, marshalJSON(e.ExtraVars, "{}"), marshalJSON(e.EnvVars, "{}"), e.SecretsBlob,
		marshalJSON(e.ExternalSecrets, "[]")).
		Scan(&e.CreatedAt, &e.UpdatedAt)
}

// UpdateEnvironment updates an environment.
func (s *Store) UpdateEnvironment(ctx context.Context, e *model.Environment) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE environments SET name=$2, description=$3, extra_vars=$4, env_vars=$5, secrets=$6,
		    external_secrets=$7, updated_at=now()
		WHERE id=$1`,
		e.ID, e.Name, e.Description, marshalJSON(e.ExtraVars, "{}"), marshalJSON(e.EnvVars, "{}"), e.SecretsBlob,
		marshalJSON(e.ExternalSecrets, "[]"))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetEnvironment returns one environment.
func (s *Store) GetEnvironment(ctx context.Context, id string) (*model.Environment, error) {
	return scanEnvironment(s.pool.QueryRow(ctx, environmentSelect+` WHERE id=$1`, id))
}

// ListEnvironments returns environments for a project plus shared ones
// (projectID empty = all).
func (s *Store) ListEnvironments(ctx context.Context, projectID string) ([]*model.Environment, error) {
	q := environmentSelect + ` ORDER BY created_at`
	args := []any{}
	if projectID != "" {
		q = environmentSelect + ` WHERE (project_id = $1 OR project_id IS NULL) ORDER BY created_at`
		args = append(args, projectID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEnvironment removes an environment.
func (s *Store) DeleteEnvironment(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM environments WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanEnvironment(row rowScanner) (*model.Environment, error) {
	var e model.Environment
	var ev, env, ext []byte
	err := row.Scan(&e.ID, &e.ProjectID, &e.Name, &e.Description, &ev, &env, &e.SecretsBlob, &ext, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.ExtraVars = unmarshalMap(ev)
	e.EnvVars = map[string]string{}
	_ = json.Unmarshal(env, &e.EnvVars)
	_ = json.Unmarshal(ext, &e.ExternalSecrets)
	return &e, nil
}

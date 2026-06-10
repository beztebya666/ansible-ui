package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const secretBackendSelect = `
	SELECT id, name, type, address, mount, namespace, insecure, region, access_key_id, tenant_id, token, created_at, updated_at
	FROM secret_backends`

// CreateSecretBackend inserts an external secret manager connection.
func (s *Store) CreateSecretBackend(ctx context.Context, b *model.SecretBackend) error {
	if b.ID == "" {
		b.ID = NewID("sb")
	}
	if b.Type == "" {
		b.Type = model.SecretBackendVault
	}
	if b.Mount == "" {
		b.Mount = "secret"
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO secret_backends (id, name, type, address, mount, namespace, insecure, region, access_key_id, tenant_id, token)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING created_at, updated_at`,
		b.ID, b.Name, b.Type, b.Address, b.Mount, b.Namespace, b.Insecure, b.Region, b.AccessKeyID, b.TenantID, b.TokenBlob).
		Scan(&b.CreatedAt, &b.UpdatedAt)
}

// UpdateSecretBackend updates a backend (TokenBlob is written as-is; callers
// pass the existing ciphertext when the token is left unchanged).
func (s *Store) UpdateSecretBackend(ctx context.Context, b *model.SecretBackend) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE secret_backends SET name=$2, type=$3, address=$4, mount=$5, namespace=$6,
		    insecure=$7, region=$8, access_key_id=$9, tenant_id=$10, token=$11, updated_at=now()
		WHERE id=$1`,
		b.ID, b.Name, b.Type, b.Address, b.Mount, b.Namespace, b.Insecure, b.Region, b.AccessKeyID, b.TenantID, b.TokenBlob)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSecretBackend returns one backend (with the encrypted token blob).
func (s *Store) GetSecretBackend(ctx context.Context, id string) (*model.SecretBackend, error) {
	return scanSecretBackend(s.pool.QueryRow(ctx, secretBackendSelect+` WHERE id=$1`, id))
}

// ListSecretBackends returns all backends.
func (s *Store) ListSecretBackends(ctx context.Context) ([]*model.SecretBackend, error) {
	rows, err := s.pool.Query(ctx, secretBackendSelect+` ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.SecretBackend
	for rows.Next() {
		b, err := scanSecretBackend(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// DeleteSecretBackend removes a backend.
func (s *Store) DeleteSecretBackend(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM secret_backends WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSecretBackend(row rowScanner) (*model.SecretBackend, error) {
	var b model.SecretBackend
	err := row.Scan(&b.ID, &b.Name, &b.Type, &b.Address, &b.Mount, &b.Namespace, &b.Insecure, &b.Region, &b.AccessKeyID, &b.TenantID, &b.TokenBlob, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.HasToken = len(b.TokenBlob) > 0
	return &b, nil
}

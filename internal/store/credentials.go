package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const credentialSelect = `
	SELECT id, project_id, owner_user_id, name, type, login, (secret IS NOT NULL), ssh_certificate, created_at, updated_at FROM credentials`

// CreateCredential stores a Key Store entry. secret is the already-encrypted
// blob (or nil).
func (s *Store) CreateCredential(ctx context.Context, c *model.Credential, secret []byte) error {
	if c.ID == "" {
		c.ID = NewID("cred")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO credentials (id, project_id, owner_user_id, name, type, login, secret, ssh_certificate)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at, updated_at`,
		c.ID, c.ProjectID, c.OwnerUserID, c.Name, c.Type, c.Login, secret, c.SSHCertificate).Scan(&c.CreatedAt, &c.UpdatedAt)
}

// UpdateCredential updates metadata, and the secret only when updateSecret.
func (s *Store) UpdateCredential(ctx context.Context, c *model.Credential, secret []byte, updateSecret bool) error {
	var ct interface{ RowsAffected() int64 }
	var err error
	if updateSecret {
		ct, err = s.pool.Exec(ctx, `UPDATE credentials SET name=$2, type=$3, login=$4, secret=$5, ssh_certificate=$6, updated_at=now() WHERE id=$1`,
			c.ID, c.Name, c.Type, c.Login, secret, c.SSHCertificate)
	} else {
		ct, err = s.pool.Exec(ctx, `UPDATE credentials SET name=$2, type=$3, login=$4, ssh_certificate=$5, updated_at=now() WHERE id=$1`,
			c.ID, c.Name, c.Type, c.Login, c.SSHCertificate)
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetCredential returns metadata for one credential.
func (s *Store) GetCredential(ctx context.Context, id string) (*model.Credential, error) {
	return scanCredential(s.pool.QueryRow(ctx, credentialSelect+` WHERE id=$1`, id))
}

// GetCredentialSecret returns the raw (encrypted) secret blob.
func (s *Store) GetCredentialSecret(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	err := s.pool.QueryRow(ctx, `SELECT secret FROM credentials WHERE id=$1`, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// ListCredentials returns credentials for a project plus shared ones (projectID
// empty = all).
// ListCredentials returns credentials visible to userID: shared + (project-scoped
// for projectID) + the user's own personal ones. Other users' personal
// credentials are never returned.
func (s *Store) ListCredentials(ctx context.Context, projectID, userID string) ([]*model.Credential, error) {
	q := credentialSelect + ` WHERE (owner_user_id IS NULL OR owner_user_id = $1)`
	args := []any{userID}
	if projectID != "" {
		q += ` AND (project_id = $2 OR project_id IS NULL)`
		args = append(args, projectID)
	}
	q += ` ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Credential
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCredential removes a credential.
func (s *Store) DeleteCredential(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM credentials WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCredential(row rowScanner) (*model.Credential, error) {
	var c model.Credential
	err := row.Scan(&c.ID, &c.ProjectID, &c.OwnerUserID, &c.Name, &c.Type, &c.Login, &c.HasSecret, &c.SSHCertificate, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// CreateToken stores an API token (only its hash is persisted).
func (s *Store) CreateToken(ctx context.Context, t *model.APIToken, tokenHash string) error {
	if t.ID == "" {
		t.ID = NewID("tok")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO api_tokens (id, user_id, name, token_hash, prefix, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at`,
		t.ID, t.UserID, t.Name, tokenHash, t.Prefix, t.ExpiresAt).Scan(&t.CreatedAt)
}

// TokenUser resolves a token hash to its (unexpired) user and token id.
func (s *Store) TokenUser(ctx context.Context, tokenHash string) (*model.User, string, error) {
	var u model.User
	var tokenID string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.role, u.created_at, t.id
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND (t.expires_at IS NULL OR t.expires_at > now())`, tokenHash).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt, &tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &u, tokenID, err
}

// TouchToken records last use (best-effort).
func (s *Store) TouchToken(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE api_tokens SET last_used_at=now() WHERE id=$1`, id)
}

// ListTokens returns a user's tokens (no secrets).
func (s *Store) ListTokens(ctx context.Context, userID string) ([]*model.APIToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, prefix, last_used_at, expires_at, created_at
		FROM api_tokens WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.APIToken
	for rows.Next() {
		var t model.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// DeleteToken removes a token owned by the user.
func (s *Store) DeleteToken(ctx context.Context, id, userID string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM api_tokens WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

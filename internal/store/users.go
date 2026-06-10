package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// CreateUser inserts a user with a pre-hashed password.
func (s *Store) CreateUser(ctx context.Context, u *model.User, passwordHash string) error {
	if u.ID == "" {
		u.ID = NewID("usr")
	}
	if u.Role == "" {
		u.Role = model.RoleUser
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO users (id, username, email, role, password_hash)
		VALUES ($1,$2,$3,$4,$5) RETURNING created_at`,
		u.ID, u.Username, u.Email, u.Role, passwordHash).Scan(&u.CreatedAt)
}

// GetUserByUsername returns a user and their password hash for login.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, string, error) {
	var u model.User
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, email, role, password_hash, two_factor_enabled, email_otp_enabled, created_at FROM users WHERE username=$1`, username).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role, &hash, &u.TwoFactorEnabled, &u.EmailOTPEnabled, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &u, hash, err
}

// UpsertExternalUser returns the user with the given username, creating it (as
// a local row with an unusable password) if it does not exist. Used by LDAP and
// OIDC so the rest of the system treats every principal uniformly.
func (s *Store) UpsertExternalUser(ctx context.Context, username, email, role string) (*model.User, error) {
	u, _, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	nu := &model.User{Username: username, Email: email, Role: role}
	if err := s.CreateUser(ctx, nu, "!external-auth-no-local-login"); err != nil {
		return nil, err
	}
	return nu, nil
}

// GetUser returns a user by id.
func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, email, role, two_factor_enabled, email_otp_enabled, created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.TwoFactorEnabled, &u.EmailOTPEnabled, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// GetUserByEmail returns a user by (non-empty) email — used for OIDC account
// linking. Returns ErrNotFound if none / no email match.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	if email == "" {
		return nil, ErrNotFound
	}
	var u model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, email, role, two_factor_enabled, email_otp_enabled, created_at
		 FROM users WHERE lower(email)=lower($1) ORDER BY created_at LIMIT 1`, email).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.TwoFactorEnabled, &u.EmailOTPEnabled, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// ListUsers returns all users.
func (s *Store) ListUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, username, email, role, two_factor_enabled, email_otp_enabled, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.TwoFactorEnabled, &u.EmailOTPEnabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &u)
	}
	return out, rows.Err()
}

// SetUserTOTP stores (encrypted) the TOTP secret + enabled flag for a user.
func (s *Store) SetUserTOTP(ctx context.Context, userID string, secret []byte, enabled bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_secret=$2, two_factor_enabled=$3 WHERE id=$1`, userID, secret, enabled)
	return err
}

// SetUserEmailOTP toggles email-OTP login for a user (clears any pending code on disable).
func (s *Store) SetUserEmailOTP(ctx context.Context, userID string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET email_otp_enabled=$2 WHERE id=$1`, userID, enabled)
	if err == nil && !enabled {
		_, _ = s.pool.Exec(ctx, `DELETE FROM email_otps WHERE user_id=$1`, userID)
	}
	return err
}

// SetEmailOTP stores (upserts) a single-use, hashed login code for a user.
func (s *Store) SetEmailOTP(ctx context.Context, userID, codeHash string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO email_otps (user_id, code_hash, expires_at) VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE SET code_hash=$2, expires_at=$3`, userID, codeHash, expires)
	return err
}

// ConsumeEmailOTP verifies + deletes a user's login code; true only if it matches
// and hasn't expired (single use).
func (s *Store) ConsumeEmailOTP(ctx context.Context, userID, codeHash string) (bool, error) {
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM email_otps WHERE user_id=$1 AND code_hash=$2 AND expires_at > now()`, userID, codeHash)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}

// GetUserTOTP returns the encrypted TOTP secret blob + enabled flag for a user.
func (s *Store) GetUserTOTP(ctx context.Context, userID string) ([]byte, bool, error) {
	var blob []byte
	var enabled bool
	err := s.pool.QueryRow(ctx,
		`SELECT totp_secret, two_factor_enabled FROM users WHERE id=$1`, userID).Scan(&blob, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrNotFound
	}
	return blob, enabled, err
}

// CountUsers reports how many users exist (for first-run setup).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// UpdateUserPassword sets a new password hash.
func (s *Store) UpdateUserPassword(ctx context.Context, id, hash string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, hash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserRole updates a user's global role (used by group→role mapping on login).
func (s *Store) SetUserRole(ctx context.Context, id, role string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET role=$2 WHERE id=$1`, id, role)
	return err
}

// DeleteUser removes a user.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- sessions ----------------------------------------------------------

// CreateSession persists an opaque session token.
func (s *Store) CreateSession(ctx context.Context, token, userID string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES ($1,$2,$3)`,
		token, userID, expires)
	return err
}

// SessionUser returns the user for a valid (unexpired) session token.
func (s *Store) SessionUser(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.role, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token=$1 AND s.expires_at > now()`, token).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token=$1`, token)
	return err
}

// PurgeExpiredSessions drops stale sessions.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	return err
}

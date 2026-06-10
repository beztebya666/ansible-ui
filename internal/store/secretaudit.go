package store

import (
	"context"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// LogSecretAccess records one access to secret material (best-effort; callers
// ignore the error so auditing never breaks the operation being audited).
func (s *Store) LogSecretAccess(ctx context.Context, l *model.SecretAccessLog) error {
	if l.ID == "" {
		l.ID = NewID("sal")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO secret_access_log (id, actor, action, credential_id, credential_name, detail)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at`,
		l.ID, l.Actor, l.Action, l.CredentialID, l.CredentialName, l.Detail).Scan(&l.CreatedAt)
}

// ListSecretAccessLogs returns the most recent secret accesses, newest first.
func (s *Store) ListSecretAccessLogs(ctx context.Context, limit int) ([]*model.SecretAccessLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor, action, credential_id, credential_name, detail, created_at
		FROM secret_access_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.SecretAccessLog{}
	for rows.Next() {
		var l model.SecretAccessLog
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.CredentialID,
			&l.CredentialName, &l.Detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// ListSecretAccessLogsBetween returns secret accesses within [from, to], oldest
// first — the complete trail for a compliance report's date range.
func (s *Store) ListSecretAccessLogsBetween(ctx context.Context, from, to time.Time) ([]*model.SecretAccessLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor, action, credential_id, credential_name, detail, created_at
		FROM secret_access_log WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.SecretAccessLog{}
	for rows.Next() {
		var l model.SecretAccessLog
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.CredentialID,
			&l.CredentialName, &l.Detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// DeleteSecretAccessLogsOlderThan trims the trail (retention sweep).
func (s *Store) DeleteSecretAccessLogsOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	ct, err := s.pool.Exec(ctx, `DELETE FROM secret_access_log WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

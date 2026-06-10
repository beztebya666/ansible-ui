package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const notificationSelect = `
	SELECT id, type, name, enabled, events, config, project_id, template, created_at, updated_at FROM notification_channels`

// CreateNotificationChannel inserts a channel.
func (s *Store) CreateNotificationChannel(ctx context.Context, c *model.NotificationChannel) error {
	if c.ID == "" {
		c.ID = NewID("ntf")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO notification_channels (id, type, name, enabled, events, config, project_id, template)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at, updated_at`,
		c.ID, c.Type, c.Name, c.Enabled, marshalJSON(c.Events, "[]"), marshalJSON(c.Config, "{}"), c.ProjectID, c.Template).
		Scan(&c.CreatedAt, &c.UpdatedAt)
}

// UpdateNotificationChannel updates a channel.
func (s *Store) UpdateNotificationChannel(ctx context.Context, c *model.NotificationChannel) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE notification_channels SET type=$2, name=$3, enabled=$4, events=$5, config=$6, project_id=$7, template=$8, updated_at=now()
		WHERE id=$1`,
		c.ID, c.Type, c.Name, c.Enabled, marshalJSON(c.Events, "[]"), marshalJSON(c.Config, "{}"), c.ProjectID, c.Template)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetNotificationChannel returns one channel.
func (s *Store) GetNotificationChannel(ctx context.Context, id string) (*model.NotificationChannel, error) {
	return scanChannel(s.pool.QueryRow(ctx, notificationSelect+` WHERE id=$1`, id))
}

// ListNotificationChannels returns all channels.
func (s *Store) ListNotificationChannels(ctx context.Context) ([]*model.NotificationChannel, error) {
	return s.queryChannels(ctx, notificationSelect+` ORDER BY created_at`)
}

// ListEnabledNotificationChannels returns enabled channels (for dispatch).
func (s *Store) ListEnabledNotificationChannels(ctx context.Context) ([]*model.NotificationChannel, error) {
	return s.queryChannels(ctx, notificationSelect+` WHERE enabled = true`)
}

// DeleteNotificationChannel removes a channel.
func (s *Store) DeleteNotificationChannel(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) queryChannels(ctx context.Context, q string, args ...any) ([]*model.NotificationChannel, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.NotificationChannel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LogNotification records one delivery attempt (best-effort; callers ignore the
// error so audit logging never breaks the dispatch path).
func (s *Store) LogNotification(ctx context.Context, l *model.NotificationLog) error {
	if l.ID == "" {
		l.ID = NewID("nlog")
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO notification_log
		  (id, channel_id, channel_name, channel_type, run_id, run_name, project_id, event, ok, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at`,
		l.ID, l.ChannelID, l.ChannelName, l.ChannelType, l.RunID, l.RunName,
		l.ProjectID, l.Event, l.OK, l.Error).Scan(&l.CreatedAt)
}

// ListNotificationLogs returns the most recent delivery attempts, newest first.
func (s *Store) ListNotificationLogs(ctx context.Context, limit int) ([]*model.NotificationLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, channel_id, channel_name, channel_type, run_id, run_name, project_id, event, ok, error, created_at
		FROM notification_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.NotificationLog{}
	for rows.Next() {
		var l model.NotificationLog
		if err := rows.Scan(&l.ID, &l.ChannelID, &l.ChannelName, &l.ChannelType, &l.RunID,
			&l.RunName, &l.ProjectID, &l.Event, &l.OK, &l.Error, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// DeleteNotificationLogsOlderThan trims the delivery log (retention sweep).
func (s *Store) DeleteNotificationLogsOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	ct, err := s.pool.Exec(ctx, `DELETE FROM notification_log WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

func scanChannel(row rowScanner) (*model.NotificationChannel, error) {
	var c model.NotificationChannel
	var events, config []byte
	err := row.Scan(&c.ID, &c.Type, &c.Name, &c.Enabled, &events, &config, &c.ProjectID, &c.Template, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(events, &c.Events)
	c.Config = map[string]string{}
	_ = json.Unmarshal(config, &c.Config)
	return &c, nil
}

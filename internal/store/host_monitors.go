package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const monitorSelect = `SELECT host, inventory_id, status, consec_fails, threshold, last_error,
	last_checked_at, last_up_at, created_at FROM host_monitors`

// EnsureHostMonitor creates a monitor for a host (no-op if it already exists).
func (s *Store) EnsureHostMonitor(ctx context.Context, host, inventoryID string, threshold int) error {
	if threshold <= 0 {
		threshold = 3
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO host_monitors (host, inventory_id, threshold) VALUES ($1,$2,$3)
		ON CONFLICT (host) DO UPDATE SET inventory_id=$2, threshold=$3`,
		host, inventoryID, threshold)
	return err
}

// UpdateHostMonitorStatus records a check result.
func (s *Store) UpdateHostMonitorStatus(ctx context.Context, host, status string, consecFails int, lastErr string, up bool) error {
	if up {
		_, err := s.pool.Exec(ctx, `
			UPDATE host_monitors SET status=$2, consec_fails=$3, last_error=$4,
			    last_checked_at=now(), last_up_at=now() WHERE host=$1`,
			host, status, consecFails, lastErr)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE host_monitors SET status=$2, consec_fails=$3, last_error=$4, last_checked_at=now() WHERE host=$1`,
		host, status, consecFails, lastErr)
	return err
}

// ListHostMonitors returns all monitors.
func (s *Store) ListHostMonitors(ctx context.Context) ([]*model.HostMonitor, error) {
	return s.queryMonitors(ctx, monitorSelect+` ORDER BY host`)
}

// ListHostMonitorsByInventory returns monitors created from one inventory.
func (s *Store) ListHostMonitorsByInventory(ctx context.Context, inventoryID string) ([]*model.HostMonitor, error) {
	return s.queryMonitors(ctx, monitorSelect+` WHERE inventory_id=$1 ORDER BY host`, inventoryID)
}

// DeleteHostMonitorsByInventory removes all monitors created from an inventory.
func (s *Store) DeleteHostMonitorsByInventory(ctx context.Context, inventoryID string) (int64, error) {
	ct, err := s.pool.Exec(ctx, `DELETE FROM host_monitors WHERE inventory_id=$1`, inventoryID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// DeleteHostMonitor removes one monitor.
func (s *Store) DeleteHostMonitor(ctx context.Context, host string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM host_monitors WHERE host=$1`, host)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) queryMonitors(ctx context.Context, q string, args ...any) ([]*model.HostMonitor, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.HostMonitor{}
	for rows.Next() {
		var m model.HostMonitor
		if err := rows.Scan(&m.Host, &m.InventoryID, &m.Status, &m.ConsecFails, &m.Threshold,
			&m.LastError, &m.LastCheckedAt, &m.LastUpAt, &m.CreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return out, nil
			}
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

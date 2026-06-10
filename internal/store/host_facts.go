package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// UpsertHostFacts stores (replacing) the latest fact snapshot for a host.
func (s *Store) UpsertHostFacts(ctx context.Context, h *model.HostFacts) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO host_facts (host, inventory_id, facts, gathered_at)
		VALUES ($1,$2,$3,now())
		ON CONFLICT (host) DO UPDATE SET inventory_id=$2, facts=$3, gathered_at=now()
		RETURNING gathered_at`,
		h.Host, h.InventoryID, marshalJSON(h.Facts, "{}")).Scan(&h.GatheredAt)
}

// ListHostFacts returns every host's stored facts, newest first.
func (s *Store) ListHostFacts(ctx context.Context) ([]*model.HostFacts, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT host, inventory_id, facts, gathered_at FROM host_facts ORDER BY gathered_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.HostFacts{}
	for rows.Next() {
		h, err := scanHostFacts(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHostFacts returns one host's stored facts.
func (s *Store) GetHostFacts(ctx context.Context, host string) (*model.HostFacts, error) {
	return scanHostFacts(s.pool.QueryRow(ctx,
		`SELECT host, inventory_id, facts, gathered_at FROM host_facts WHERE host=$1`, host))
}

// DeleteHostFacts removes a host from the registry.
func (s *Store) DeleteHostFacts(ctx context.Context, host string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM host_facts WHERE host=$1`, host)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanHostFacts(row rowScanner) (*model.HostFacts, error) {
	var h model.HostFacts
	var facts []byte
	err := row.Scan(&h.Host, &h.InventoryID, &facts, &h.GatheredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	h.Facts = map[string]any{}
	_ = json.Unmarshal(facts, &h.Facts)
	return &h, nil
}

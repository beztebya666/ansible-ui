package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

const integrationSelect = `
	SELECT i.id, i.template_id, i.workflow_id, i.name, i.token, i.active, i.auth_method, i.auth_header,
	       i.auth_secret, i.pass_payload, i.aliases, i.last_triggered_at, i.created_at,
	       COALESCE(t.name, w.name, ''), COALESCE(t.project_id, w.project_id, '')
	FROM integrations i
	LEFT JOIN templates t ON t.id = i.template_id
	LEFT JOIN workflows w ON w.id = i.workflow_id`

// CreateIntegration inserts a webhook integration.
func (s *Store) CreateIntegration(ctx context.Context, in *model.Integration) error {
	if in.ID == "" {
		in.ID = NewID("whk")
	}
	if in.AuthMethod == "" {
		in.AuthMethod = model.AuthNone
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO integrations (id, template_id, workflow_id, name, token, active, auth_method, auth_header,
		    auth_secret, pass_payload, aliases)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING created_at`,
		in.ID, nilIfEmpty(in.TemplateID), nilIfEmpty(in.WorkflowID), in.Name, in.Token, in.Active, in.AuthMethod, in.AuthHeader,
		in.AuthSecretBlob, in.PassPayload, marshalJSON(in.Aliases, "[]")).Scan(&in.CreatedAt)
}

// UpdateIntegration updates a webhook integration's settings.
func (s *Store) UpdateIntegration(ctx context.Context, in *model.Integration) error {
	if in.AuthMethod == "" {
		in.AuthMethod = model.AuthNone
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE integrations SET name=$2, active=$3, auth_method=$4, auth_header=$5,
		    auth_secret=$6, pass_payload=$7, aliases=$8 WHERE id=$1`,
		in.ID, in.Name, in.Active, in.AuthMethod, in.AuthHeader, in.AuthSecretBlob,
		in.PassPayload, marshalJSON(in.Aliases, "[]"))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetIntegration returns one integration by id.
func (s *Store) GetIntegration(ctx context.Context, id string) (*model.Integration, error) {
	return scanIntegration(s.pool.QueryRow(ctx, integrationSelect+` WHERE i.id=$1`, id))
}

// GetIntegrationByToken resolves a webhook token or alias (must be active).
func (s *Store) GetIntegrationByToken(ctx context.Context, token string) (*model.Integration, error) {
	return scanIntegration(s.pool.QueryRow(ctx,
		integrationSelect+` WHERE (i.token=$1 OR i.aliases @> to_jsonb($1::text)) AND i.active=true`, token))
}

// ListIntegrations returns integrations (optionally for one template).
func (s *Store) ListIntegrations(ctx context.Context, templateID string) ([]*model.Integration, error) {
	q := integrationSelect + ` ORDER BY i.created_at`
	args := []any{}
	if templateID != "" {
		q = integrationSelect + ` WHERE i.template_id=$1 ORDER BY i.created_at`
		args = append(args, templateID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Integration
	for rows.Next() {
		in, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// TouchIntegration records a trigger.
func (s *Store) TouchIntegration(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE integrations SET last_triggered_at=now() WHERE id=$1`, id)
}

// DeleteIntegration removes an integration.
func (s *Store) DeleteIntegration(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM integrations WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanIntegration(row rowScanner) (*model.Integration, error) {
	var in model.Integration
	var aliases []byte
	var tid, wid *string
	err := row.Scan(&in.ID, &tid, &wid, &in.Name, &in.Token, &in.Active, &in.AuthMethod,
		&in.AuthHeader, &in.AuthSecretBlob, &in.PassPayload, &aliases, &in.LastTriggeredAt,
		&in.CreatedAt, &in.TemplateName, &in.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if tid != nil {
		in.TemplateID = *tid
	}
	if wid != nil {
		in.WorkflowID = *wid
	}
	_ = json.Unmarshal(aliases, &in.Aliases)
	return &in, nil
}

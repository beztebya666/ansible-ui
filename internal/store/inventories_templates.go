package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// ---- inventories -------------------------------------------------------

const inventorySelect = `SELECT id, project_id, name, type, content, provider, credential_id, region, runner_tag, conn_credential_ids, created_at, updated_at FROM inventories`

// CreateInventory inserts an inventory.
func (s *Store) CreateInventory(ctx context.Context, inv *model.Inventory) error {
	if inv.ID == "" {
		inv.ID = NewID("inv")
	}
	if inv.Type == "" {
		inv.Type = model.InventoryStatic
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO inventories (id, project_id, name, type, content, provider, credential_id, region, runner_tag, conn_credential_ids)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING created_at, updated_at`,
		inv.ID, inv.ProjectID, inv.Name, inv.Type, inv.Content, inv.Provider, inv.CredentialID, inv.Region, inv.RunnerTag, marshalJSON(inv.ConnCredentialIDs, "[]")).
		Scan(&inv.CreatedAt, &inv.UpdatedAt)
}

// UpdateInventory updates name/type/content.
func (s *Store) UpdateInventory(ctx context.Context, inv *model.Inventory) error {
	if inv.Type == "" {
		inv.Type = model.InventoryStatic
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE inventories SET name=$2, type=$3, content=$4, provider=$5, credential_id=$6, region=$7, runner_tag=$8, conn_credential_ids=$9, updated_at=now() WHERE id=$1`,
		inv.ID, inv.Name, inv.Type, inv.Content, inv.Provider, inv.CredentialID, inv.Region, inv.RunnerTag, marshalJSON(inv.ConnCredentialIDs, "[]"))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetInventory returns one inventory.
func (s *Store) GetInventory(ctx context.Context, id string) (*model.Inventory, error) {
	return scanInventory(s.pool.QueryRow(ctx, inventorySelect+` WHERE id=$1`, id))
}

// ListInventories returns inventories for a project.
func (s *Store) ListInventories(ctx context.Context, projectID string) ([]*model.Inventory, error) {
	rows, err := s.pool.Query(ctx, inventorySelect+` WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Inventory
	for rows.Next() {
		inv, err := scanInventory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// DeleteInventory removes an inventory.
func (s *Store) DeleteInventory(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM inventories WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanInventory(row rowScanner) (*model.Inventory, error) {
	var inv model.Inventory
	var connCreds []byte
	err := row.Scan(&inv.ID, &inv.ProjectID, &inv.Name, &inv.Type, &inv.Content, &inv.Provider, &inv.CredentialID, &inv.Region, &inv.RunnerTag, &connCreds, &inv.CreatedAt, &inv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(connCreds, &inv.ConnCredentialIDs)
	return &inv, nil
}

// ---- templates ---------------------------------------------------------

const templateSelect = `
	SELECT id, project_id, name, description, app, action, playbook, inventory_id, vault_credential_id,
	       environment_id, survey_vars, limit_pattern, tags, skip_tags, extra_vars,
	       check_mode, diff_mode, verbosity,
	       cli_args, prompts, allow_parallel, autorun_on_commit, suppress_success_notifications,
	       workspace, auto_approve, vaults, type, build_template_id, runner_tag, requires_approval,
	       artifact_paths, suppress_all_notifications, inventory_ids, max_concurrent, tf_backend, created_at, updated_at
	FROM templates`

// CreateTemplate inserts a template.
func (s *Store) CreateTemplate(ctx context.Context, t *model.Template) error {
	if t.ID == "" {
		t.ID = NewID("tpl")
	}
	ev := marshalJSON(t.ExtraVars, "{}")
	sv := marshalJSON(t.SurveyVars, "[]")
	if t.App == "" {
		t.App = model.AppAnsible
	}
	if t.Type == "" {
		t.Type = model.TemplateTypeTask
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO templates (id, project_id, name, description, app, action, playbook, inventory_id,
		    vault_credential_id, environment_id, survey_vars, limit_pattern, tags, skip_tags,
		    extra_vars, check_mode, diff_mode, verbosity,
		    cli_args, prompts, allow_parallel, autorun_on_commit, suppress_success_notifications,
		    workspace, auto_approve, vaults, type, build_template_id, runner_tag, requires_approval, artifact_paths,
		    suppress_all_notifications, inventory_ids, max_concurrent, tf_backend)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35)
		RETURNING created_at, updated_at`,
		t.ID, t.ProjectID, t.Name, t.Description, t.App, t.Action, t.Playbook, t.InventoryID, t.VaultCredentialID,
		t.EnvironmentID, sv, t.Limit, t.Tags, t.SkipTags, ev, t.Check, t.Diff, t.Verbosity,
		marshalJSON(t.CliArgs, "[]"), marshalJSON(t.Prompts, "{}"), t.AllowParallel, t.AutorunOnCommit,
		t.SuppressSuccessNotifications, t.Workspace, t.AutoApprove, marshalJSON(t.Vaults, "[]"), t.Type, t.BuildTemplateID, t.RunnerTag, t.RequiresApproval,
		marshalJSON(t.ArtifactPaths, "[]"), t.SuppressAllNotifications, marshalJSON(t.InventoryIDs, "[]"), t.MaxConcurrent, t.TfBackend).
		Scan(&t.CreatedAt, &t.UpdatedAt)
}

// UpdateTemplate updates a template.
func (s *Store) UpdateTemplate(ctx context.Context, t *model.Template) error {
	ev := marshalJSON(t.ExtraVars, "{}")
	if t.App == "" {
		t.App = model.AppAnsible
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE templates SET name=$2, description=$3, app=$4, action=$5, playbook=$6, inventory_id=$7,
		    vault_credential_id=$8, environment_id=$9, survey_vars=$10, limit_pattern=$11, tags=$12,
		    skip_tags=$13, extra_vars=$14, check_mode=$15, diff_mode=$16, verbosity=$17,
		    cli_args=$18, prompts=$19, allow_parallel=$20, autorun_on_commit=$21,
		    suppress_success_notifications=$22, workspace=$23, auto_approve=$24, vaults=$25,
		    type=$26, build_template_id=$27, runner_tag=$28, requires_approval=$29, artifact_paths=$30,
		    suppress_all_notifications=$31, inventory_ids=$32, max_concurrent=$33, tf_backend=$34, updated_at=now()
		WHERE id=$1`,
		t.ID, t.Name, t.Description, t.App, t.Action, t.Playbook, t.InventoryID, t.VaultCredentialID,
		t.EnvironmentID, marshalJSON(t.SurveyVars, "[]"), t.Limit, t.Tags,
		t.SkipTags, ev, t.Check, t.Diff, t.Verbosity,
		marshalJSON(t.CliArgs, "[]"), marshalJSON(t.Prompts, "{}"), t.AllowParallel, t.AutorunOnCommit,
		t.SuppressSuccessNotifications, t.Workspace, t.AutoApprove, marshalJSON(t.Vaults, "[]"), t.Type, t.BuildTemplateID, t.RunnerTag, t.RequiresApproval,
		marshalJSON(t.ArtifactPaths, "[]"), t.SuppressAllNotifications, marshalJSON(t.InventoryIDs, "[]"), t.MaxConcurrent, t.TfBackend)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTemplate returns one template.
func (s *Store) GetTemplate(ctx context.Context, id string) (*model.Template, error) {
	return scanTemplate(s.pool.QueryRow(ctx, templateSelect+` WHERE id=$1`, id))
}

// ListTemplates returns templates for a project (or all if projectID == "").
func (s *Store) ListTemplates(ctx context.Context, projectID string) ([]*model.Template, error) {
	q := templateSelect + ` ORDER BY created_at`
	args := []any{}
	if projectID != "" {
		q = templateSelect + ` WHERE project_id=$1 ORDER BY created_at`
		args = append(args, projectID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTemplate removes a template.
func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM templates WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTemplate(row rowScanner) (*model.Template, error) {
	var t model.Template
	var ev, sv, cliArgs, prompts, vaults, artifactPaths, invIDs []byte
	err := row.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &t.App, &t.Action, &t.Playbook, &t.InventoryID,
		&t.VaultCredentialID, &t.EnvironmentID, &sv, &t.Limit, &t.Tags, &t.SkipTags, &ev,
		&t.Check, &t.Diff, &t.Verbosity,
		&cliArgs, &prompts, &t.AllowParallel, &t.AutorunOnCommit, &t.SuppressSuccessNotifications,
		&t.Workspace, &t.AutoApprove, &vaults, &t.Type, &t.BuildTemplateID, &t.RunnerTag, &t.RequiresApproval,
		&artifactPaths, &t.SuppressAllNotifications, &invIDs, &t.MaxConcurrent, &t.TfBackend, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.ExtraVars = unmarshalMap(ev)
	_ = json.Unmarshal(sv, &t.SurveyVars)
	_ = json.Unmarshal(cliArgs, &t.CliArgs)
	_ = json.Unmarshal(prompts, &t.Prompts)
	_ = json.Unmarshal(vaults, &t.Vaults)
	_ = json.Unmarshal(artifactPaths, &t.ArtifactPaths)
	_ = json.Unmarshal(invIDs, &t.InventoryIDs)
	return &t, nil
}

// ---- json helpers ------------------------------------------------------

func marshalJSON(v any, fallback string) []byte {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return []byte(fallback)
	}
	return b
}

func unmarshalMap(b []byte) map[string]any {
	m := map[string]any{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

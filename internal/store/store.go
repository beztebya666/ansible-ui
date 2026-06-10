// Package store is the Postgres persistence layer for the api service.
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikiv/ansible-ui/internal/model"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// Connect opens a pool (retrying until Postgres is ready) and applies the schema.
func Connect(ctx context.Context, url string) (*Store, error) {
	var pool *pgxpool.Pool
	var err error
	deadline := time.Now().Add(60 * time.Second)
	for {
		pool, err = pgxpool.New(ctx, url)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("postgres not ready: %w", err)
		}
		time.Sleep(time.Second)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, schemaSQL)
	return err
}

// NewID returns a short, unique, prefixed identifier (e.g. run_9f3a...).
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// ---- projects ----------------------------------------------------------

// CreateProject inserts a project.
func (s *Store) CreateProject(ctx context.Context, p *model.Project) error {
	if p.ID == "" {
		p.ID = NewID("prj")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO projects (id, slug, name, description, path, source_type, git_url, git_branch, repository_id, sub_path)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, p.Slug, p.Name, p.Description, p.Path, p.SourceType, p.GitURL, p.GitBranch, p.RepositoryID, p.SubPath)
	return err
}

// GetProjectBySlug returns a project by slug or ErrNotFound.
func (s *Store) GetProjectBySlug(ctx context.Context, slug string) (*model.Project, error) {
	return s.scanProject(s.pool.QueryRow(ctx, projectSelect+` WHERE slug=$1`, slug))
}

// GetProject returns a project by id or ErrNotFound.
func (s *Store) GetProject(ctx context.Context, id string) (*model.Project, error) {
	return s.scanProject(s.pool.QueryRow(ctx, projectSelect+` WHERE id=$1`, id))
}

// ListProjects returns all projects, newest first.
func (s *Store) ListProjects(ctx context.Context) ([]*model.Project, error) {
	rows, err := s.pool.Query(ctx, projectSelect+` ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Project
	for rows.Next() {
		p, err := s.scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProject updates a project's editable fields (name, description, slug/tag).
// The on-disk path is tracked separately, so changing the slug is just a relabel.
func (s *Store) UpdateProject(ctx context.Context, id, name, description, slug string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE projects SET name=$2, description=$3, slug=$4, updated_at=now() WHERE id=$1`,
		id, name, description, slug)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProject removes a project and its children.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const projectSelect = `
	SELECT id, slug, name, description, path, source_type, git_url, git_branch,
	       repository_id, sub_path, created_at, updated_at
	FROM projects`

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanProject(row rowScanner) (*model.Project, error) {
	var p model.Project
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Path, &p.SourceType,
		&p.GitURL, &p.GitBranch, &p.RepositoryID, &p.SubPath, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS projects (
  id          text PRIMARY KEY,
  slug        text UNIQUE NOT NULL,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  path        text NOT NULL,
  source_type text NOT NULL DEFAULT 'local',
  git_url     text NOT NULL DEFAULT '',
  git_branch  text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventories (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  content     text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS templates (
  id            text PRIMARY KEY,
  project_id    text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name          text NOT NULL,
  description   text NOT NULL DEFAULT '',
  playbook      text NOT NULL,
  inventory_id  text REFERENCES inventories(id) ON DELETE SET NULL,
  limit_pattern text NOT NULL DEFAULT '',
  tags          text NOT NULL DEFAULT '',
  skip_tags     text NOT NULL DEFAULT '',
  extra_vars    jsonb NOT NULL DEFAULT '{}',
  check_mode    boolean NOT NULL DEFAULT false,
  diff_mode     boolean NOT NULL DEFAULT false,
  verbosity     int NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS runs (
  id            text PRIMARY KEY,
  project_id    text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  template_id   text REFERENCES templates(id) ON DELETE SET NULL,
  name          text NOT NULL DEFAULT '',
  playbook      text NOT NULL,
  status        text NOT NULL DEFAULT 'pending',
  exit_code     int,
  limit_pattern text NOT NULL DEFAULT '',
  tags          text NOT NULL DEFAULT '',
  skip_tags     text NOT NULL DEFAULT '',
  extra_vars    jsonb NOT NULL DEFAULT '{}',
  check_mode    boolean NOT NULL DEFAULT false,
  diff_mode     boolean NOT NULL DEFAULT false,
  verbosity     int NOT NULL DEFAULT 0,
  args          jsonb NOT NULL DEFAULT '[]',
  stats         jsonb NOT NULL DEFAULT '{}',
  output        bytea,
  created_at    timestamptz NOT NULL DEFAULT now(),
  started_at    timestamptz,
  finished_at   timestamptz
);
CREATE INDEX IF NOT EXISTS runs_project_idx ON runs(project_id);
CREATE INDEX IF NOT EXISTS runs_created_idx ON runs(created_at DESC);

CREATE TABLE IF NOT EXISTS credentials (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  type        text NOT NULL,
  login       text NOT NULL DEFAULT '',
  secret      bytea,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repositories (
  id            text PRIMARY KEY,
  name          text NOT NULL,
  git_url       text NOT NULL,
  branch        text NOT NULL DEFAULT 'main',
  credential_id text REFERENCES credentials(id) ON DELETE SET NULL,
  status        text NOT NULL DEFAULT 'unknown',
  last_commit   text NOT NULL DEFAULT '',
  last_error    text NOT NULL DEFAULT '',
  last_synced_at timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id            text PRIMARY KEY,
  username      text UNIQUE NOT NULL,
  email         text NOT NULL DEFAULT '',
  role          text NOT NULL DEFAULT 'user',
  password_hash text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
  token      text PRIMARY KEY,
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);

CREATE TABLE IF NOT EXISTS api_tokens (
  id           text PRIMARY KEY,
  user_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         text NOT NULL DEFAULT '',
  token_hash   text UNIQUE NOT NULL,
  prefix       text NOT NULL DEFAULT '',
  last_used_at timestamptz,
  expires_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS api_tokens_user_idx ON api_tokens(user_id);

CREATE TABLE IF NOT EXISTS environments (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  extra_vars  jsonb NOT NULL DEFAULT '{}',
  env_vars    jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS schedules (
  id          text PRIMARY KEY,
  template_id text NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  name        text NOT NULL DEFAULT '',
  cron        text NOT NULL,
  active      boolean NOT NULL DEFAULT true,
  last_run_at timestamptz,
  next_run_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS schedules_template_idx ON schedules(template_id);

CREATE TABLE IF NOT EXISTS integrations (
  id                text PRIMARY KEY,
  template_id       text NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  name              text NOT NULL DEFAULT '',
  token             text UNIQUE NOT NULL,
  active            boolean NOT NULL DEFAULT true,
  last_triggered_at timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS integrations_token_idx ON integrations(token);

CREATE TABLE IF NOT EXISTS app_settings (
  key   text PRIMARY KEY,
  value text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS applications (
  id         text PRIMARY KEY,
  name       text NOT NULL,
  icon       text NOT NULL DEFAULT '',
  bin        text NOT NULL DEFAULT '',
  args       jsonb NOT NULL DEFAULT '[]',
  priority   int NOT NULL DEFAULT 0,
  active     boolean NOT NULL DEFAULT true,
  kind       text NOT NULL DEFAULT 'custom',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS activity (
  id         text PRIMARY KEY,
  actor      text NOT NULL DEFAULT '',
  action     text NOT NULL,
  target     text NOT NULL DEFAULT '',
  detail     text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS activity_created_idx ON activity(created_at DESC);

-- repo-backing columns on projects (additive, safe to re-run)
ALTER TABLE projects ADD COLUMN IF NOT EXISTS repository_id text REFERENCES repositories(id) ON DELETE SET NULL;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sub_path text NOT NULL DEFAULT '';
-- vault credential, environment and survey on templates
ALTER TABLE templates ADD COLUMN IF NOT EXISTS vault_credential_id text REFERENCES credentials(id) ON DELETE SET NULL;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS environment_id text REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS survey_vars jsonb NOT NULL DEFAULT '[]';
-- environment reference on runs
ALTER TABLE runs ADD COLUMN IF NOT EXISTS environment_id text REFERENCES environments(id) ON DELETE SET NULL;
-- multi-app: execution backend on templates and runs
ALTER TABLE templates ADD COLUMN IF NOT EXISTS app text NOT NULL DEFAULT 'ansible';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS action text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS app text NOT NULL DEFAULT 'ansible';
-- cross-replica cancel: a non-owning replica sets this; the owning replica polls + cancels
ALTER TABLE runs ADD COLUMN IF NOT EXISTS cancel_requested boolean NOT NULL DEFAULT false;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS action text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS commit_hash text NOT NULL DEFAULT '';
-- inventory types: static (inline) | file | dynamic
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS type text NOT NULL DEFAULT 'static';
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '';
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS credential_id text REFERENCES credentials(id) ON DELETE SET NULL;
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT '';
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS runner_tag text NOT NULL DEFAULT '';
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS conn_credential_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
-- environment secrets (encrypted blob, like the key store)
ALTER TABLE environments ADD COLUMN IF NOT EXISTS secrets bytea;
-- integration auth methods, payload passthrough and aliases
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS auth_method text NOT NULL DEFAULT 'none';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS auth_header text NOT NULL DEFAULT '';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS auth_secret bytea;
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS pass_payload boolean NOT NULL DEFAULT false;
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS aliases jsonb NOT NULL DEFAULT '[]';
-- An integration can trigger a workflow instead of a template (one of the two).
ALTER TABLE integrations ALTER COLUMN template_id DROP NOT NULL;
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS workflow_id text REFERENCES workflows(id) ON DELETE CASCADE;
-- one-time ("run once") schedules
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS once boolean NOT NULL DEFAULT false;
-- multi-tenant: scope shared resources to a project (NULL = shared across all)
ALTER TABLE credentials ADD COLUMN IF NOT EXISTS project_id text REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE credentials ADD COLUMN IF NOT EXISTS owner_user_id text REFERENCES users(id) ON DELETE CASCADE;
-- SSH certificate: an OpenSSH user cert (public, CA-signed) presented alongside the key
ALTER TABLE credentials ADD COLUMN IF NOT EXISTS ssh_certificate text NOT NULL DEFAULT '';
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS project_id text REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE environments ADD COLUMN IF NOT EXISTS project_id text REFERENCES projects(id) ON DELETE CASCADE;
-- NB: template_views is created further down, so its project_id ALTER lives there
-- (running it here would fail on a fresh DB — the table doesn't exist yet).
-- template parity (CLI args, prompts, flags, workspace/auto-approve, multi-vault)
ALTER TABLE templates ADD COLUMN IF NOT EXISTS cli_args jsonb NOT NULL DEFAULT '[]';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS prompts jsonb NOT NULL DEFAULT '{}';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS allow_parallel boolean NOT NULL DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS autorun_on_commit boolean NOT NULL DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS suppress_success_notifications boolean NOT NULL DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS suppress_all_notifications boolean NOT NULL DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS inventory_ids jsonb NOT NULL DEFAULT '[]';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS max_concurrent int NOT NULL DEFAULT 0;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS tf_backend text NOT NULL DEFAULT '';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS workspace text NOT NULL DEFAULT '';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS auto_approve boolean NOT NULL DEFAULT false;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS vaults jsonb NOT NULL DEFAULT '[]';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS type text NOT NULL DEFAULT 'task';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS build_template_id text REFERENCES templates(id) ON DELETE SET NULL;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS version text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS cli_args jsonb NOT NULL DEFAULT '[]';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS workspace text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS auto_approve boolean NOT NULL DEFAULT false;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS triggered_by text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS log_times jsonb NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS template_views (
  id         text PRIMARY KEY,
  name       text NOT NULL,
  app        text NOT NULL DEFAULT '',
  search     text NOT NULL DEFAULT '',
  position   int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE template_views ADD COLUMN IF NOT EXISTS project_id text REFERENCES projects(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS notification_channels (
  id         text PRIMARY KEY,
  type       text NOT NULL,
  name       text NOT NULL DEFAULT '',
  enabled    boolean NOT NULL DEFAULT true,
  events     jsonb NOT NULL DEFAULT '[]',
  config     jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS template text NOT NULL DEFAULT '';

-- Per-project RBAC: a user's role within a project (viewer | editor | admin).
-- A project with no members is unrestricted (any signed-in user); once members
-- exist, non-members are read-only. Global admins always have full access.
CREATE TABLE IF NOT EXISTS project_members (
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'viewer',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, user_id)
);
CREATE INDEX IF NOT EXISTS project_members_user_idx ON project_members(user_id);

-- External secret managers (HashiCorp Vault, …). The auth token is encrypted at rest.
CREATE TABLE IF NOT EXISTS secret_backends (
  id         text PRIMARY KEY,
  name       text NOT NULL,
  type       text NOT NULL DEFAULT 'vault',
  address    text NOT NULL DEFAULT '',
  mount      text NOT NULL DEFAULT 'secret',
  namespace  text NOT NULL DEFAULT '',
  insecure   boolean NOT NULL DEFAULT false,
  token      bytea,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- AWS Secrets Manager backends: region + access key id (the secret access key
-- reuses the encrypted token column).
ALTER TABLE secret_backends ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT '';
ALTER TABLE secret_backends ADD COLUMN IF NOT EXISTS access_key_id text NOT NULL DEFAULT '';
ALTER TABLE secret_backends ADD COLUMN IF NOT EXISTS tenant_id text NOT NULL DEFAULT '';

ALTER TABLE environments ADD COLUMN IF NOT EXISTS external_secrets jsonb NOT NULL DEFAULT '[]';

-- Distributed runners: execution agents that register + heartbeat. A run is
-- dispatched to a runner advertising the template's runner_tag; the built-in
-- runner (the configured RUNNER_URL) handles untagged runs.
CREATE TABLE IF NOT EXISTS runners (
  id           text PRIMARY KEY,
  name         text NOT NULL DEFAULT '',
  url          text NOT NULL,
  tags         jsonb NOT NULL DEFAULT '[]',
  platform     text NOT NULL DEFAULT '',
  version      text NOT NULL DEFAULT '',
  max_concurrent int NOT NULL DEFAULT 0,
  builtin      boolean NOT NULL DEFAULT false,
  last_seen_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS runners_url_idx ON runners(url);
ALTER TABLE runners ADD COLUMN IF NOT EXISTS max_concurrent int NOT NULL DEFAULT 0;

ALTER TABLE templates ADD COLUMN IF NOT EXISTS runner_tag text NOT NULL DEFAULT '';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS requires_approval boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret bytea;
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_otp_enabled boolean NOT NULL DEFAULT false;
-- Single-use email-OTP login codes (hashed, short-lived).
CREATE TABLE IF NOT EXISTS email_otps (
  user_id    text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  code_hash  text NOT NULL,
  expires_at timestamptz NOT NULL
);
ALTER TABLE templates ADD COLUMN IF NOT EXISTS artifact_paths jsonb NOT NULL DEFAULT '[]';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS artifact_paths jsonb NOT NULL DEFAULT '[]';
-- HA: the AES-encrypted launch envelope (exec spec + repo-sync spec) lets the
-- leader dispatch a queued/awaiting run on any replica, surviving restarts.
ALTER TABLE runs ADD COLUMN IF NOT EXISTS launch_envelope bytea;
CREATE INDEX IF NOT EXISTS runs_queued_idx ON runs(created_at) WHERE status = 'queued';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS workflow_run_id text;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS workflow_step int NOT NULL DEFAULT 0;

-- Workflows: a sequential pipeline of templates with conditional steps.
CREATE TABLE IF NOT EXISTS workflows (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  steps       jsonb NOT NULL DEFAULT '[]',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS workflow_runs (
  id          text PRIMARY KEY,
  workflow_id text NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL DEFAULT '',
  triggered_by text NOT NULL DEFAULT '',
  status      text NOT NULL DEFAULT 'running',
  steps       jsonb NOT NULL DEFAULT '[]',
  created_at  timestamptz NOT NULL DEFAULT now(),
  started_at  timestamptz,
  finished_at timestamptz
);
CREATE INDEX IF NOT EXISTS workflow_runs_wf_idx ON workflow_runs(workflow_id, created_at DESC);
ALTER TABLE workflows ADD COLUMN IF NOT EXISTS variables jsonb NOT NULL DEFAULT '{}';
ALTER TABLE workflow_runs ADD COLUMN IF NOT EXISTS variables jsonb NOT NULL DEFAULT '{}';
-- Workflow versioning: an immutable snapshot of a workflow's definition before
-- each edit/rollback, so changes can be reviewed + rolled back.
CREATE TABLE IF NOT EXISTS workflow_versions (
  id          text PRIMARY KEY,
  workflow_id text NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  version     int NOT NULL,
  name        text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  steps       jsonb NOT NULL DEFAULT '[]',
  variables   jsonb NOT NULL DEFAULT '{}',
  actor       text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS workflow_versions_wf_idx ON workflow_versions(workflow_id, version DESC);
-- A schedule can target a workflow instead of a template (one of the two).
ALTER TABLE schedules ALTER COLUMN template_id DROP NOT NULL;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS workflow_id text REFERENCES workflows(id) ON DELETE CASCADE;
-- Notification channels can be scoped to one project (NULL = all projects).
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS project_id text;

-- Files captured from a run's working directory after it finished.
CREATE TABLE IF NOT EXISTS run_artifacts (
  id         text PRIMARY KEY,
  run_id     text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  name       text NOT NULL,
  size       bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS run_artifacts_run_idx ON run_artifacts(run_id);

-- Branches the UI committed + pushed for git-backed projects (for the PR panel).
CREATE TABLE IF NOT EXISTS pushed_branches (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch      text NOT NULL,
  commit_hash text NOT NULL DEFAULT '',
  pr_url      text NOT NULL DEFAULT '',
  files       jsonb NOT NULL DEFAULT '[]',
  actor       text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS pushed_branches_project_idx ON pushed_branches(project_id);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS runner_id text NOT NULL DEFAULT '';
-- AES-encrypted JSON of the run's resolved secret extra-vars; revealed to admins
-- on demand (the run record/log themselves only ever show masked values).
ALTER TABLE runs ADD COLUMN IF NOT EXISTS secret_vars bytea;

-- Notification delivery log: one row per attempted dispatch to a channel, so
-- admins can audit who was notified about what and whether delivery succeeded.
CREATE TABLE IF NOT EXISTS notification_log (
  id           text PRIMARY KEY,
  channel_id   text NOT NULL DEFAULT '',
  channel_name text NOT NULL DEFAULT '',
  channel_type text NOT NULL DEFAULT '',
  run_id       text NOT NULL DEFAULT '',
  run_name     text NOT NULL DEFAULT '',
  project_id   text NOT NULL DEFAULT '',
  event        text NOT NULL DEFAULT '',
  ok           boolean NOT NULL DEFAULT true,
  error        text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notification_log_created_idx ON notification_log(created_at DESC);

-- Secret access audit trail: one row per access to secret material (an admin
-- revealing a run's secret vars, deriving a key's public half, or a run
-- consuming a Key Store credential). Security-sensitive, kept separate from the
-- general activity feed so it can be audited on its own.
CREATE TABLE IF NOT EXISTS secret_access_log (
  id              text PRIMARY KEY,
  actor           text NOT NULL DEFAULT '',
  action          text NOT NULL DEFAULT '',
  credential_id   text NOT NULL DEFAULT '',
  credential_name text NOT NULL DEFAULT '',
  detail          text NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS secret_access_log_created_idx ON secret_access_log(created_at DESC);

-- Persistent repository cache: when on, the leader background-syncs the repo's
-- mirror so its file tree / detection stays warm without waiting for a run.
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS cache_enabled boolean NOT NULL DEFAULT true;

-- Custom roles: admin-defined project roles with a granular capability set
-- (subset of view/run/edit/manage). A project_members.role may name one by id.
CREATE TABLE IF NOT EXISTS custom_roles (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  permissions jsonb NOT NULL DEFAULT '[]',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Central host registry + fact storage: the latest gathered Ansible facts per
-- host (keyed by host name; re-gathering replaces the snapshot).
CREATE TABLE IF NOT EXISTS host_facts (
  host         text PRIMARY KEY,
  inventory_id text NOT NULL DEFAULT '',
  facts        jsonb NOT NULL DEFAULT '{}',
  gathered_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS host_facts_gathered_idx ON host_facts(gathered_at DESC);

-- Inventory monitoring: a per-host reachability monitor (created from an
-- inventory). The leader periodically pings these and alerts on up→down.
CREATE TABLE IF NOT EXISTS host_monitors (
  host            text PRIMARY KEY,
  inventory_id    text NOT NULL DEFAULT '',
  status          text NOT NULL DEFAULT 'unknown',
  consec_fails    int NOT NULL DEFAULT 0,
  threshold       int NOT NULL DEFAULT 3,
  last_error      text NOT NULL DEFAULT '',
  last_checked_at timestamptz,
  last_up_at      timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
`

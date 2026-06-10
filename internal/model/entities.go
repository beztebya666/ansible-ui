package model

import "time"

// Credential types in the Key Store.
const (
	CredentialSSH      = "ssh"            // SSH private key (+ optional passphrase)
	CredentialPassword = "login_password" // username + password
	CredentialVault    = "vault"          // ansible-vault password
	CredentialGCP      = "gcp"            // GCP service-account JSON key
	CredentialAzure    = "azure"          // Azure service principal (JSON in ServiceAccount)
)

// Credential is a Key Store entry. Secret material is never serialised to the
// client — only metadata and a HasSecret flag.
type Credential struct {
	ID             string    `json:"id"`
	ProjectID      *string   `json:"projectId,omitempty"`   // nil = shared across all projects
	OwnerUserID    *string   `json:"ownerUserId,omitempty"` // set = personal (visible only to this user)
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Login          string    `json:"login,omitempty"` // username for login_password
	HasSecret      bool      `json:"hasSecret"`
	SSHCertificate string    `json:"sshCertificate,omitempty"` // OpenSSH user cert (public; signed by a CA) presented alongside the SSH key
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// CredentialSecret is the decrypted secret material (server-side only).
type CredentialSecret struct {
	SSHPrivateKey  string `json:"sshPrivateKey,omitempty"`
	Passphrase     string `json:"passphrase,omitempty"`
	Password       string `json:"password,omitempty"`
	VaultPassword  string `json:"vaultPassword,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"` // GCP service-account JSON
}

// Repository sync states.
const (
	RepoUnknown = "unknown"
	RepoSyncing = "syncing"
	RepoReady   = "ready"
	RepoError   = "error"
)

// Repository is a Git source that can back many projects (one per subfolder).
type Repository struct {
	ID           string     `json:"id"`
	ProjectID    *string    `json:"projectId,omitempty"` // nil = shared
	Name         string     `json:"name"`
	GitURL       string     `json:"gitUrl"`
	Branch       string     `json:"branch"`
	CredentialID *string    `json:"credentialId,omitempty"`
	Status       string     `json:"status"`
	LastCommit   string     `json:"lastCommit,omitempty"`
	LastError    string     `json:"lastError,omitempty"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
	CacheEnabled bool       `json:"cacheEnabled"` // keep a warm mirror, background-synced by the leader
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// Environment secret kinds: injected as a process env var, or as an extra-var.
const (
	EnvSecretEnv = "env"
	EnvSecretVar = "var"
)

// EnvSecret is an encrypted key/value attached to an Environment (Semaphore's
// Variable-Group "Secrets" tab). Values are never returned to the client.
type EnvSecret struct {
	Name     string `json:"name"`
	Type     string `json:"type"`            // env | var
	Value    string `json:"value,omitempty"` // input only; omitted on read
	HasValue bool   `json:"hasValue,omitempty"`
}

// ExternalSecretRef points an env var / extra-var at a secret living in an
// external manager (e.g. HashiCorp Vault). The value is fetched fresh at launch
// — nothing secret is stored locally, only the reference.
type ExternalSecretRef struct {
	Name    string `json:"name"`            // env var / extra-var to inject
	Backend string `json:"backend"`         // secret backend id
	Path    string `json:"path"`            // secret path within the backend (e.g. "app/db")
	Field   string `json:"field,omitempty"` // key within the secret (empty = whole JSON)
	AsVar   bool   `json:"asVar,omitempty"` // true → extra-var, false → process env var
}

// Environment is a reusable set of extra-vars (-e), process environment
// variables, and encrypted secrets, applied to a run via a template.
type Environment struct {
	ID              string              `json:"id"`
	ProjectID       *string             `json:"projectId,omitempty"` // nil = shared
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	ExtraVars       map[string]any      `json:"extraVars"`       // passed as --extra-vars
	EnvVars         map[string]string   `json:"envVars"`         // set in the process environment
	Secrets         []EnvSecret         `json:"secrets"`         // encrypted at rest; values hidden on read
	SecretsBlob     []byte              `json:"-"`               // ciphertext, persisted by the store
	ExternalSecrets []ExternalSecretRef `json:"externalSecrets"` // resolved from a backend at launch
	CreatedAt       time.Time           `json:"createdAt"`
	UpdatedAt       time.Time           `json:"updatedAt"`
}

// Secret backend types (external secret managers).
const (
	SecretBackendVault    = "vault"    // HashiCorp Vault KV v2
	SecretBackendAWS      = "aws"      // AWS Secrets Manager
	SecretBackendAzure    = "azure"    // Azure Key Vault
	SecretBackendGCP      = "gcp"      // GCP Secret Manager
	SecretBackendCyberArk = "cyberark" // CyberArk Conjur (REST)
)

// SecretBackend is a connection to an external secret manager. The secret
// credential is encrypted at rest (AES-256-GCM) and never returned to the client.
// Several fields are reused across backends:
//   - Address: vault URL (vault) · custom endpoint (aws) · key-vault URL (azure)
//   - AccessKeyID: IAM access key id (aws) · AAD client id (azure) · project id (gcp)
//   - Token (secret): vault token · aws secret access key · azure client secret ·
//     gcp service-account JSON
type SecretBackend struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`                  // vault | aws | azure | gcp
	Address     string `json:"address"`               // see per-type note above
	Mount       string `json:"mount,omitempty"`       // vault: KV v2 mount (default "secret")
	Namespace   string `json:"namespace,omitempty"`   // vault: Enterprise namespace
	Insecure    bool   `json:"insecure,omitempty"`    // vault: skip TLS verification (self-signed)
	Region      string `json:"region,omitempty"`      // aws: e.g. eu-central-1
	TenantID    string `json:"tenantId,omitempty"`    // azure: AAD tenant id
	AccessKeyID string `json:"accessKeyId,omitempty"` // aws key id / azure client id / gcp project id
	// Secret credential (input only):
	Token     string    `json:"token,omitempty"`
	HasToken  bool      `json:"hasToken"` // read-side: a credential is stored
	TokenBlob []byte    `json:"-"`        // ciphertext, persisted by the store
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Survey variable types.
const (
	SurveyText   = "text"
	SurveyInt    = "int"
	SurveyEnum   = "enum"
	SurveySecret = "secret"
)

// SurveyVar is an operator-prompted input defined on a template. At launch the
// answers are merged into the run's extra-vars.
type SurveyVar struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"` // for enum
}

// Integration webhook auth methods (Semaphore parity).
const (
	AuthNone      = "none"
	AuthToken     = "token"     // header value must equal the secret
	AuthHMAC      = "hmac"      // hex HMAC-SHA256 of body in the configured header
	AuthGitHub    = "github"    // X-Hub-Signature-256: sha256=<hmac>
	AuthBitbucket = "bitbucket" // X-Hub-Signature: sha256=<hmac>
	AuthBasic     = "basic"     // Authorization: Basic base64(secret) where secret="user:pass"
)

// Integration is an inbound webhook that triggers a template when POSTed to.
type Integration struct {
	ID              string     `json:"id"`
	TemplateID      string     `json:"templateId"`
	WorkflowID      string     `json:"workflowId,omitempty"` // set instead of TemplateID to trigger a workflow
	Name            string     `json:"name"`
	Token           string     `json:"token"` // appears in the webhook URL
	Active          bool       `json:"active"`
	AuthMethod      string     `json:"authMethod"`           // none | token | hmac | github | bitbucket | basic
	AuthHeader      string     `json:"authHeader,omitempty"` // header to read for token/hmac
	PassPayload     bool       `json:"passPayload"`          // pass the JSON body as the `webhook` extra-var
	Aliases         []string   `json:"aliases,omitempty"`    // alternative tokens that resolve here
	AuthSecret      string     `json:"authSecret,omitempty"` // input only; never returned
	HasSecret       bool       `json:"hasSecret"`            // output only
	AuthSecretBlob  []byte     `json:"-"`                    // encrypted secret at rest
	LastTriggeredAt *time.Time `json:"lastTriggeredAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`

	// Hydrated, not persisted:
	TemplateName string `json:"templateName,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
}

// Schedule triggers a template on a cron expression, or once at a set time.
type Schedule struct {
	ID         string     `json:"id"`
	TemplateID string     `json:"templateId"`
	WorkflowID string     `json:"workflowId,omitempty"` // set instead of TemplateID for a workflow schedule
	Name       string     `json:"name"`
	Cron       string     `json:"cron"`
	Once       bool       `json:"once"` // fire a single time at NextRunAt, then deactivate
	Active     bool       `json:"active"`
	LastRunAt  *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt  *time.Time `json:"nextRunAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`

	// Hydrated, not persisted:
	TemplateName string `json:"templateName,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
}

// Notification channel types.
const (
	NotifyTelegram   = "telegram"
	NotifySlack      = "slack"
	NotifyWebhook    = "webhook"
	NotifyEmail      = "email"
	NotifyDiscord    = "discord"
	NotifyTeams      = "teams"      // Microsoft Teams (incoming webhook)
	NotifyGotify     = "gotify"     // self-hosted Gotify
	NotifyRocketchat = "rocketchat" // Rocket.Chat incoming webhook
	NotifyNtfy       = "ntfy"       // ntfy.sh / self-hosted topic
	NotifyGoogleChat = "googlechat" // Google Chat incoming webhook
	NotifyPushover   = "pushover"
	NotifyDingtalk   = "dingtalk"
	NotifyPagerDuty  = "pagerduty" // incident-management (Events API v2)
	NotifyOpsgenie   = "opsgenie"  // incident-management (Alert API v2)
)

// NotificationChannel delivers run-completion alerts to an external service.
type NotificationChannel struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // telegram | slack | webhook
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Events    []string          `json:"events"`              // run statuses to notify on; empty = all terminal
	Config    map[string]string `json:"config"`              // telegram: botToken,chatId · slack/webhook: url
	ProjectID *string           `json:"projectId,omitempty"` // scope to one project; nil/empty = all projects
	Template  string            `json:"template,omitempty"`  // optional message body template (vars: {{run}} {{status}} …)
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// SecretAccessLog is one access to secret material — an admin revealing a run's
// secret vars, deriving a credential's public key, or a run consuming a Key Store
// credential. Recorded separately from the activity feed for security auditing.
type SecretAccessLog struct {
	ID             string    `json:"id"`
	Actor          string    `json:"actor"`
	Action         string    `json:"action"` // run-secrets | pubkey | credential-use
	CredentialID   string    `json:"credentialId,omitempty"`
	CredentialName string    `json:"credentialName,omitempty"`
	Detail         string    `json:"detail,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// NotificationLog is one attempted delivery of a notification to a channel,
// recorded for auditing (who was alerted about which run, and whether it
// reached the channel). OK=false carries the failure reason in Error.
type NotificationLog struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channelId"`
	ChannelName string    `json:"channelName"`
	ChannelType string    `json:"channelType"`
	RunID       string    `json:"runId"`
	RunName     string    `json:"runName"`
	ProjectID   string    `json:"projectId"`
	Event       string    `json:"event"` // the run status that triggered it
	OK          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Application is an execution backend shown in the app picker. The 7 built-ins
// are seeded (kind=builtin); admins can toggle/configure them and add custom
// apps (kind=custom) that run an arbitrary binary in the PTY.
type Application struct {
	ID        string    `json:"id"`       // e.g. ansible, terraform, or a custom slug
	Name      string    `json:"name"`     // display name
	Icon      string    `json:"icon"`     // lucide icon key (frontend)
	Bin       string    `json:"bin"`      // custom apps: binary/path to execute
	Args      []string  `json:"args"`     // base args prepended to every run
	Priority  int       `json:"priority"` // sort order in the picker
	Active    bool      `json:"active"`   // shown/usable
	Kind      string    `json:"kind"`     // builtin | custom
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Application kinds.
const (
	AppKindBuiltin = "builtin"
	AppKindCustom  = "custom"
)

// Activity is one audit/activity-feed entry.
type Activity struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

// User roles.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Per-project roles (RBAC within a project), ordered by privilege.
const (
	ProjectRoleViewer = "viewer" // read-only
	ProjectRoleEditor = "editor" // run + edit files/templates/inventories
	ProjectRoleAdmin  = "admin"  // + manage members / project settings
)

// ProjectRoleRank ranks a project role for "at least" comparisons (higher = more).
func ProjectRoleRank(role string) int {
	switch role {
	case ProjectRoleAdmin:
		return 3
	case ProjectRoleEditor:
		return 2
	case ProjectRoleViewer:
		return 1
	default:
		return 0
	}
}

// Project capabilities — the granular permissions a role grants. Custom roles
// declare an arbitrary subset; the three built-ins map to fixed sets below.
const (
	CapView   = "view"   // see the project, its templates/runs/inventories
	CapRun    = "run"    // launch runs / approve / cancel
	CapEdit   = "edit"   // create/edit templates, inventories, files, schedules, workflows
	CapManage = "manage" // manage members, settings, custom-role assignment
)

// AllCaps is every capability (what a global/project admin holds).
var AllCaps = []string{CapView, CapRun, CapEdit, CapManage}

// builtinRoleCaps maps the three built-in project roles to their capability sets.
var builtinRoleCaps = map[string][]string{
	ProjectRoleViewer: {CapView},
	ProjectRoleEditor: {CapView, CapRun, CapEdit},
	ProjectRoleAdmin:  {CapView, CapRun, CapEdit, CapManage},
}

// BuiltinRoleCaps returns the caps for a built-in role (nil if not built-in).
func BuiltinRoleCaps(role string) []string { return builtinRoleCaps[role] }

// CustomRole is an admin-defined project role with a granular capability set.
// Members can be assigned a custom role by id (stored in project_members.role).
type CustomRole struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Permissions []string  `json:"permissions"` // subset of AllCaps
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// User is a local account.
type User struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	Role             string    `json:"role"`
	TwoFactorEnabled bool      `json:"twoFactorEnabled"`
	EmailOTPEnabled  bool      `json:"emailOtpEnabled"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ProjectMember is a user's role within a project (per-project RBAC).
type ProjectMember struct {
	ProjectID string    `json:"projectId"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// APIToken is a bearer token for programmatic API access (CI, automation).
// The secret is shown once at creation and only its hash is stored.
type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"` // first chars, for display
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`

	// Returned only once, on creation:
	Token string `json:"token,omitempty"`
}

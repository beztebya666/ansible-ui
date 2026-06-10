// Package model holds the domain types shared across the api and runner
// services: persisted entities plus the wire protocol used to stream a live
// ansible-playbook execution.
package model

import "time"

// Run lifecycle states.
const (
	StatusAwaiting = "awaiting" // held for manual approval before it may run
	StatusQueued   = "queued"   // waiting for a runner slot in its pool to free up
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// IsTerminalStatus reports whether a run status is final (no further streaming).
func IsTerminalStatus(s string) bool {
	return s == StatusSuccess || s == StatusFailed || s == StatusCanceled
}

// Apps are the execution backends a template/run can target. Ansible is the
// default; the rest make this a multi-app control plane (Semaphore parity+).
const (
	AppAnsible    = "ansible"
	AppTerraform  = "terraform"
	AppTofu       = "tofu"
	AppTerragrunt = "terragrunt"
	AppPulumi     = "pulumi"
	AppBash       = "bash"
	AppPowerShell = "powershell"
	AppPython     = "python"
)

// IsInfraApp reports whether app is a Terraform-family backend (init + action).
func IsInfraApp(app string) bool {
	return app == AppTerraform || app == AppTofu || app == AppTerragrunt
}

// IsScriptApp reports whether app runs a single script file.
func IsScriptApp(app string) bool {
	return app == AppBash || app == AppPowerShell || app == AppPython
}

// Project is a directory of Ansible content (playbooks, roles, inventories).
type Project struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"` // relative to the data root (local projects)
	SourceType  string `json:"sourceType"`
	GitURL      string `json:"gitUrl,omitempty"`
	GitBranch   string `json:"gitBranch,omitempty"`
	// Repo-backed projects: working dir = <repo checkout>/<subPath>.
	RepositoryID *string   `json:"repositoryId,omitempty"`
	SubPath      string    `json:"subPath"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	// Hydrated, not persisted:
	Playbooks      []string `json:"playbooks,omitempty"`
	InventoryFiles []string `json:"inventoryFiles,omitempty"`
	RepositoryName string   `json:"repositoryName,omitempty"`
	MyRole         string   `json:"myRole,omitempty"` // current user's effective role in this project
}

// Inventory types (Semaphore parity).
const (
	InventoryStatic  = "static"  // inline content (INI/YAML), written to a temp file
	InventoryFile    = "file"    // a path to an inventory file inside the project
	InventoryDynamic = "dynamic" // a path to an executable (dynamic) inventory script
	InventoryCloud   = "cloud"   // turnkey cloud source: the runner generates the plugin config
	InventoryURL     = "url"     // Content holds an HTTP(S) URL fetched at launch into a temp file
)

// Cloud inventory providers (ansible inventory plugins).
const (
	CloudProviderAWSEC2     = "aws_ec2"     // amazon.aws.aws_ec2
	CloudProviderGCPCompute = "gcp_compute" // google.cloud.gcp_compute
	CloudProviderAzureRM    = "azure_rm"    // azure.azcollection.azure_rm
)

// Inventory is a named inventory bound to a project. For static inventories
// Content holds the inline text; for file/dynamic it holds a project-relative
// path; for cloud it holds optional extra plugin YAML and Provider/CredentialID/
// Region drive the generated config.
type Inventory struct {
	ID           string  `json:"id"`
	ProjectID    string  `json:"projectId"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Content      string  `json:"content"`
	Provider     string  `json:"provider,omitempty"`     // cloud: aws_ec2 (…)
	CredentialID *string `json:"credentialId,omitempty"` // cloud: Key Store creds for the provider
	Region       string  `json:"region,omitempty"`       // cloud: e.g. eu-central-1
	RunnerTag    string  `json:"runnerTag,omitempty"`    // affinity: pin runs using this inventory to a runner advertising this tag
	// Non-cloud inventories: Key Store SSH credential ids whose keys ansible uses to
	// connect to this inventory's hosts (loaded into an ssh-agent; multiple = tried in turn).
	ConnCredentialIDs []string  `json:"connCredentialIds"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// TemplatePrompts marks which fields the operator is asked for at launch.
type TemplatePrompts struct {
	CliArgs   bool `json:"cliArgs"`
	Branch    bool `json:"branch"`
	Inventory bool `json:"inventory"`
	Limit     bool `json:"limit"`
	Tags      bool `json:"tags"`
	SkipTags  bool `json:"skipTags"`
	Debug     bool `json:"debug"`
	Vaults    bool `json:"vaults"` // prompt the operator to choose vault credentials at launch
}

// Template is a reusable run configuration (Semaphore's "task template").
type Template struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"projectId"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	App               string         `json:"app"`              // ansible | terraform | tofu | terragrunt | bash | powershell | python
	Action            string         `json:"action,omitempty"` // infra apps: plan | apply | destroy
	Playbook          string         `json:"playbook"`         // ansible: playbook · infra: working subdir · script: script path
	InventoryID       *string        `json:"inventoryId,omitempty"`
	InventoryIDs      []string       `json:"inventoryIds"` // curated extra inventories selectable at launch
	VaultCredentialID *string        `json:"vaultCredentialId,omitempty"`
	EnvironmentID     *string        `json:"environmentId,omitempty"`
	SurveyVars        []SurveyVar    `json:"surveyVars"`
	Limit             string         `json:"limit"`
	Tags              string         `json:"tags"`
	SkipTags          string         `json:"skipTags"`
	ExtraVars         map[string]any `json:"extraVars"`
	Check             bool           `json:"check"`
	Diff              bool           `json:"diff"`
	Verbosity         int            `json:"verbosity"`
	// Semaphore-parity advanced options:
	CliArgs                      []string        `json:"cliArgs"`         // extra raw args appended to the command
	Prompts                      TemplatePrompts `json:"prompts"`         // which fields to ask the operator at launch
	AllowParallel                bool            `json:"allowParallel"`   // allow concurrent runs of this template
	MaxConcurrent                int             `json:"maxConcurrent"`   // cap on simultaneous runs of this template (0 = unlimited)
	AutorunOnCommit              bool            `json:"autorunOnCommit"` // run when the repo gets a new commit
	SuppressSuccessNotifications bool            `json:"suppressSuccessNotifications"`
	SuppressAllNotifications     bool            `json:"suppressAllNotifications"` // mute every notification for this template
	Workspace                    string          `json:"workspace,omitempty"`      // infra: terraform/tofu workspace
	AutoApprove                  bool            `json:"autoApprove"`              // infra: apply/destroy without prompt
	Vaults                       []string        `json:"vaults"`                   // credential ids (multi-vault)
	// Template type (Semaphore parity): task (default), build (produces a numbered
	// artifact), deploy (consumes the latest successful build's artifact).
	Type             string    `json:"type"`
	BuildTemplateID  *string   `json:"buildTemplateId,omitempty"` // deploy: which build template it deploys
	RunnerTag        string    `json:"runnerTag,omitempty"`       // dispatch to a runner advertising this tag (empty = default)
	RequiresApproval bool      `json:"requiresApproval"`          // launches wait for a project admin to approve before running
	ArtifactPaths    []string  `json:"artifactPaths"`             // globs (relative to the working dir) captured after each run
	TfBackend        string    `json:"tfBackend,omitempty"`       // infra: HTTP state-backend address (empty = local/default backend)
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	// Hydrated, not persisted: the version of the latest successful build this
	// template is associated with (its own, for build; the consumed build's, for
	// deploy) — the Semaphore-style VERSION shown in the list.
	LastBuildVersion string `json:"lastBuildVersion,omitempty"`
	// Hydrated, not persisted: the owning project's name (for the cross-project
	// workflow-step picker, which lists templates across projects).
	ProjectName string `json:"projectName,omitempty"`
}

// Template types.
const (
	TemplateTypeTask   = "task"
	TemplateTypeBuild  = "build"
	TemplateTypeDeploy = "deploy"
)

// TemplateView is a saved filter/tab over the templates list (Semaphore "Views").
type TemplateView struct {
	ID        string    `json:"id"`
	ProjectID *string   `json:"projectId,omitempty"` // nil = shared
	Name      string    `json:"name"`
	App       string    `json:"app"`    // filter by app (empty = any)
	Search    string    `json:"search"` // free-text filter on name/playbook
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RunStats is the parsed PLAY RECAP, persisted as jsonb.
type RunStats struct {
	Hosts       int `json:"hosts"`
	Ok          int `json:"ok"`
	Changed     int `json:"changed"`
	Unreachable int `json:"unreachable"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
	Rescued     int `json:"rescued"`
	Ignored     int `json:"ignored"`
}

// Run is one execution of a playbook.
type Run struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"projectId"`
	TemplateID    *string        `json:"templateId,omitempty"`
	EnvironmentID *string        `json:"environmentId,omitempty"`
	Name          string         `json:"name"`
	TriggeredBy   string         `json:"triggeredBy,omitempty"` // user / scheduler / webhook
	App           string         `json:"app"`
	Action        string         `json:"action,omitempty"`
	Commit        string         `json:"commit,omitempty"`     // git commit the run executed against
	Version       string         `json:"version,omitempty"`    // build artifact number (build runs) / deployed build (deploy runs)
	RunnerID      string         `json:"runnerId,omitempty"`   // which runner executed it (empty = built-in)
	RunnerName    string         `json:"runnerName,omitempty"` // hydrated, not persisted
	Playbook      string         `json:"playbook"`
	Status        string         `json:"status"`
	ExitCode      *int           `json:"exitCode,omitempty"`
	Limit         string         `json:"limit"`
	Tags          string         `json:"tags"`
	SkipTags      string         `json:"skipTags"`
	ExtraVars     map[string]any `json:"extraVars"`
	Check         bool           `json:"check"`
	Diff          bool           `json:"diff"`
	Verbosity     int            `json:"verbosity"`
	CliArgs       []string       `json:"cliArgs,omitempty"`
	Workspace     string         `json:"workspace,omitempty"`
	ArtifactPaths []string       `json:"artifactPaths,omitempty"` // globs captured after the run (from the template)
	AutoApprove   bool           `json:"autoApprove,omitempty"`
	Args          []string       `json:"args"`
	Stats         RunStats       `json:"stats"`
	WorkflowRunID *string        `json:"workflowRunId,omitempty"` // set when this run is a workflow step
	WorkflowStep  int            `json:"workflowStep,omitempty"`  // its index in the workflow run
	CreatedAt     time.Time      `json:"createdAt"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`

	// AES-encrypted JSON of the resolved secret extra-vars (real values). Persisted
	// but never serialized to clients; an admin-only endpoint decrypts it on demand.
	SecretVarsBlob []byte `json:"-"`

	// Hydrated, not persisted on the list view:
	ProjectName string `json:"projectName,omitempty"`
}

// Workflow step conditions, evaluated against the pipeline state so far.
const (
	WFCondAlways    = "always"     // run regardless
	WFCondOnSuccess = "on_success" // run only while no prior step has failed
	WFCondOnFailure = "on_failure" // run only after a prior step failed (rollback / notify)
)

// Workflow run statuses.
const (
	WFStatusRunning  = "running"
	WFStatusSuccess  = "success"
	WFStatusFailed   = "failed"
	WFStatusCanceled = "canceled"
)

// WorkflowStep is one stage of a pipeline: a template to run, gated by a condition.
type WorkflowStep struct {
	Name              string `json:"name"`
	TemplateID        string `json:"templateId"`
	Condition         string `json:"condition"`                   // always | on_success | on_failure
	When              string `json:"when,omitempty"`              // extra guard vs workflow vars: key | !key | key==v | key!=v
	InventoryID       string `json:"inventoryId,omitempty"`       // per-step override of the template's inventory
	EnvironmentID     string `json:"environmentId,omitempty"`     // per-step override of the template's environment
	VaultCredentialID string `json:"vaultCredentialId,omitempty"` // per-step override of the template's vault credential(s)
	Parallel          bool   `json:"parallel,omitempty"`          // run concurrently with the preceding step(s) (same wave)
}

// Workflow chains templates into a sequential pipeline with conditional steps.
type Workflow struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"projectId"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Steps       []WorkflowStep    `json:"steps"`
	Variables   map[string]string `json:"variables"` // defaults injected as extra-vars into every step
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// WorkflowVersion is an immutable snapshot of a workflow's definition, captured
// before each edit/rollback so changes can be reviewed and rolled back.
type WorkflowVersion struct {
	ID          string            `json:"id"`
	WorkflowID  string            `json:"workflowId"`
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Steps       []WorkflowStep    `json:"steps"`
	Variables   map[string]string `json:"variables"`
	Actor       string            `json:"actor,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// WorkflowRunStep is a step's state within one workflow execution.
type WorkflowRunStep struct {
	Name              string `json:"name"`
	TemplateID        string `json:"templateId"`
	Condition         string `json:"condition"`
	When              string `json:"when,omitempty"`
	InventoryID       string `json:"inventoryId,omitempty"`
	EnvironmentID     string `json:"environmentId,omitempty"`
	VaultCredentialID string `json:"vaultCredentialId,omitempty"`
	Parallel          bool   `json:"parallel,omitempty"`
	RunID             string `json:"runId,omitempty"` // the launched run (empty until/unless it runs)
	Status            string `json:"status"`          // pending | running | success | failed | skipped
}

// WorkflowRun is one execution of a workflow.
type WorkflowRun struct {
	ID          string            `json:"id"`
	WorkflowID  string            `json:"workflowId"`
	ProjectID   string            `json:"projectId"`
	Name        string            `json:"name"`
	TriggeredBy string            `json:"triggeredBy,omitempty"`
	Status      string            `json:"status"` // running | success | failed | canceled
	Steps       []WorkflowRunStep `json:"steps"`
	Variables   map[string]any    `json:"variables,omitempty"` // effective vars (defaults + launch overrides) injected into steps
	CreatedAt   time.Time         `json:"createdAt"`
	StartedAt   *time.Time        `json:"startedAt,omitempty"`
	FinishedAt  *time.Time        `json:"finishedAt,omitempty"`
	// Hydrated, not persisted:
	WorkflowName string `json:"workflowName,omitempty"`
	ProjectName  string `json:"projectName,omitempty"`
}

// DayBucket is one day's run counts by outcome (for the insights chart).
type DayBucket struct {
	Date    string `json:"date"` // YYYY-MM-DD
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
	Other   int    `json:"other"` // canceled / other terminal
	Total   int    `json:"total"`
}

// Insights is the aggregated run analytics over a recent window.
type Insights struct {
	Days           int               `json:"days"`
	PerDay         []DayBucket       `json:"perDay"`
	Total          int               `json:"total"`
	SuccessRate    float64           `json:"successRate"`    // 0..100 of finished runs
	AvgDurationSec float64           `json:"avgDurationSec"` // started→finished
	AvgWaitSec     float64           `json:"avgWaitSec"`     // created→started (dispatch wait)
	ByTemplate     []TemplateInsight `json:"byTemplate"`     // per-template SLO breakdown (busiest first)
}

// TemplateInsight is a per-template rollup over the insights window.
type TemplateInsight struct {
	TemplateID     string  `json:"templateId"`
	TemplateName   string  `json:"templateName"`
	Runs           int     `json:"runs"`
	SuccessRate    float64 `json:"successRate"`
	AvgDurationSec float64 `json:"avgDurationSec"`
}

// RunArtifact is a file captured from a run's working directory after it finished
// (per the template's artifact globs). The bytes live on disk under
// DataDir/artifacts/<runID>/<name>; this row is the index entry.
type RunArtifact struct {
	ID        string    `json:"id"`
	RunID     string    `json:"runId"`
	Name      string    `json:"name"` // path relative to the working dir
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// Runner is an execution agent registered with the control plane. A run is
// dispatched to a runner advertising the template's RunnerTag; with no tag the
// built-in runner handles it. Status is derived from the last heartbeat.
type Runner struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"` // ws base, e.g. ws://host:8081
	Tags          []string   `json:"tags"`
	Platform      string     `json:"platform,omitempty"`
	Version       string     `json:"version,omitempty"`
	MaxConcurrent int        `json:"maxConcurrent"` // cap on simultaneous jobs (0 = unlimited)
	Builtin       bool       `json:"builtin"`
	Status        string     `json:"status"` // hydrated: online | offline
	LastSeenAt    *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// RunnerOnlineWindow is how long after its last heartbeat a runner stays "online".
const RunnerOnlineWindow = 45 * time.Second

// ---- runner wire protocol ----------------------------------------------

// ExecSpec is what the api sends the runner to start an execution. The runner
// builds the ansible-playbook command line from it and runs it inside a PTY.
type ExecSpec struct {
	RunID          string            `json:"runId"`
	App            string            `json:"app"`       // execution backend (default ansible)
	AppBin         string            `json:"appBin"`    // custom app: binary to execute (overrides dispatch)
	AppArgs        []string          `json:"appArgs"`   // custom app: base args before cliArgs/entrypoint
	Action         string            `json:"action"`    // infra apps: plan | apply | destroy
	Dir            string            `json:"dir"`       // absolute working directory
	Playbook       string            `json:"playbook"`  // ansible: playbook · infra: subdir · script: script path
	Inventory      string            `json:"inventory"` // -i value (optional)
	Limit          string            `json:"limit"`
	Tags           string            `json:"tags"`
	SkipTags       string            `json:"skipTags"`
	ExtraVars      map[string]any    `json:"extraVars"`
	Check          bool              `json:"check"`
	Diff           bool              `json:"diff"`
	Verbosity      int               `json:"verbosity"`
	CliArgs        []string          `json:"cliArgs"`     // extra raw args appended to the command
	Workspace      string            `json:"workspace"`   // infra workspace
	AutoApprove    bool              `json:"autoApprove"` // infra apply/destroy auto-approve
	Env            map[string]string `json:"env"`
	Cols           int               `json:"cols"`
	Rows           int               `json:"rows"`
	GalaxyInstall  bool              `json:"galaxyInstall"`            // install requirements before the play
	GalaxyArgs     []string          `json:"galaxyArgs,omitempty"`     // extra ansible-galaxy CLI args (private-server auth via Env)
	VaultPassword  string            `json:"vaultPassword"`            // written to a temp file → --vault-password-file
	VaultPasswords []string          `json:"vaultPasswords"`           // multi-vault → multiple --vault-password-file
	SSHPrivateKeys []string          `json:"sshPrivateKeys,omitempty"` // inventory connection keys → each written to a 0600 temp file (loaded via ssh-agent; ansible tries each)
	SSHUser        string            `json:"sshUser,omitempty"`        // inventory connection user → --user
	ArtifactPaths  []string          `json:"artifactPaths,omitempty"`  // globs captured from Dir + sent back after the run
	TimeoutSec     int               `json:"timeoutSec,omitempty"`     // max run duration; the runner cancels the process when exceeded (0 = unlimited)
	TfBackend      string            `json:"tfBackend,omitempty"`      // infra: HTTP state-backend address (terraform/tofu); empty = local
	// Git-backed runs: surfaced in the run-log preamble (repo URL/branch + commit message).
	RepoURL    string `json:"repoUrl,omitempty"`
	RepoBranch string `json:"repoBranch,omitempty"`
	CommitMsg  string `json:"commitMsg,omitempty"`
}

// GitSyncSpec asks the runner to clone/pull a repository.
type GitSyncSpec struct {
	RepoID         string `json:"repoId"`
	GitURL         string `json:"gitUrl"`
	Branch         string `json:"branch"`
	SSHPrivateKey  string `json:"sshPrivateKey,omitempty"`
	SSHCertificate string `json:"sshCertificate,omitempty"` // OpenSSH user cert presented via -o CertificateFile
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
}

// GitSyncResult is the runner's reply after a sync.
type GitSyncResult struct {
	Commit  string `json:"commit"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// InventoryListSpec asks the runner to resolve an inventory's hosts/groups by
// running `ansible-inventory --list` (so dynamic/plugin inventories resolve the
// same way they would at run time). InlineContent (when set) is written to
// Filename inside Dir first, for static/inline inventories.
type InventoryListSpec struct {
	Dir           string            `json:"dir"`           // project working dir (ansible.cfg, group_vars live here)
	InventoryArg  string            `json:"inventoryArg"`  // -i value (path relative to Dir, or a written filename)
	InlineContent string            `json:"inlineContent"` // static inventory text to materialise before listing
	Filename      string            `json:"filename"`      // where to write InlineContent (relative to Dir)
	Env           map[string]string `json:"env,omitempty"` // extra process env (cloud provider creds, etc.)
}

// InventoryListResult is the runner's reply: the raw `ansible-inventory --list`
// JSON (stdout) or an error with the captured stderr.
type InventoryListResult struct {
	JSON  string `json:"json"`
	Error string `json:"error,omitempty"`
}

// FactsGatherSpec asks the runner to gather Ansible facts (`ansible <pattern>
// -m setup --tree`) for an inventory's hosts. InlineContent (static inventories)
// is materialised to Filename inside Dir first; Pattern defaults to "all".
type FactsGatherSpec struct {
	Dir           string            `json:"dir"`
	InventoryArg  string            `json:"inventoryArg"`
	InlineContent string            `json:"inlineContent"`
	Filename      string            `json:"filename"`
	Pattern       string            `json:"pattern"`
	Env           map[string]string `json:"env,omitempty"`
}

// FactsGatherResult maps host name → that host's gathered facts (the contents of
// each --tree file's "ansible_facts"); Error carries captured stderr on failure.
type FactsGatherResult struct {
	Hosts map[string]map[string]any `json:"hosts"`
	Error string                    `json:"error,omitempty"`
}

// PingSpec asks the runner to check reachability (`ansible <pattern> -m ping`)
// for an inventory's hosts. InlineContent (static inventories) is materialised
// to Filename inside Dir first; Pattern defaults to "all".
type PingSpec struct {
	Dir           string            `json:"dir"`
	InventoryArg  string            `json:"inventoryArg"`
	InlineContent string            `json:"inlineContent"`
	Filename      string            `json:"filename"`
	Pattern       string            `json:"pattern"`
	Env           map[string]string `json:"env,omitempty"`
}

// PingResult maps host name → reachable. Error carries captured stderr on a
// total failure (e.g. inventory unparsable).
type PingResult struct {
	Reachable map[string]bool `json:"reachable"`
	Error     string          `json:"error,omitempty"`
}

// Host monitor statuses.
const (
	MonitorUnknown = "unknown"
	MonitorUp      = "up"
	MonitorDown    = "down"
)

// HostMonitor tracks a host's reachability over time (inventory monitoring).
type HostMonitor struct {
	Host          string     `json:"host"`
	InventoryID   string     `json:"inventoryId"`
	Status        string     `json:"status"` // up | down | unknown
	ConsecFails   int        `json:"consecFails"`
	Threshold     int        `json:"threshold"` // consecutive failures before "down" + alert
	LastError     string     `json:"lastError,omitempty"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	LastUpAt      *time.Time `json:"lastUpAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// HostFacts is the stored fact snapshot for one host (central host registry).
type HostFacts struct {
	Host        string         `json:"host"`
	InventoryID string         `json:"inventoryId,omitempty"`
	Facts       map[string]any `json:"facts,omitempty"`
	GatheredAt  time.Time      `json:"gatheredAt"`
	// Hydrated summary fields (extracted from Facts for the registry table):
	OS     string `json:"os,omitempty"`
	Distro string `json:"distro,omitempty"`
	IP     string `json:"ip,omitempty"`
	Kernel string `json:"kernel,omitempty"`
}

// GitFile is one file written into the working tree before a commit.
type GitFile struct {
	Path    string `json:"path"` // repo-relative path
	Content string `json:"content"`
}

// GitCommitSpec asks the runner to write files, commit them onto a NEW branch and
// push it — never onto the (often protected) base branch.
type GitCommitSpec struct {
	RepoID         string    `json:"repoId"`
	GitURL         string    `json:"gitUrl"`
	BaseBranch     string    `json:"baseBranch"` // branch the change starts from
	NewBranch      string    `json:"newBranch"`  // branch to commit onto + push
	Message        string    `json:"message"`
	AuthorName     string    `json:"authorName"`
	AuthorEmail    string    `json:"authorEmail"`
	Files          []GitFile `json:"files"`
	SSHPrivateKey  string    `json:"sshPrivateKey,omitempty"`
	SSHCertificate string    `json:"sshCertificate,omitempty"` // OpenSSH user cert presented via -o CertificateFile
	Username       string    `json:"username,omitempty"`
	Password       string    `json:"password,omitempty"`
}

// GitCommitResult is the runner's reply after a commit+push.
type GitCommitResult struct {
	Commit string `json:"commit"`
	Branch string `json:"branch"`
	Pushed bool   `json:"pushed"`
	Error  string `json:"error,omitempty"`
}

// PushedBranch records a branch the UI committed + pushed for a git-backed
// project, so the operator can find it again and open its PR/MR.
type PushedBranch struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Branch    string    `json:"branch"`
	Commit    string    `json:"commit"`
	PRURL     string    `json:"prUrl,omitempty"`
	Files     []string  `json:"files"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"createdAt"`
}

// Frame types streamed runner → api → browser.
const (
	FrameStarted  = "started"
	FrameStdout   = "stdout"
	FrameExit     = "exit"
	FrameError    = "error"
	FrameArtifact = "artifact" // runner → api: a captured file (Name + base64 Data)
	// browser/api → runner control frames:
	FrameResize = "resize"
	FrameCancel = "cancel"
)

// Frame is a single message on the streaming socket.
type Frame struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"` // base64 raw bytes for stdout / artifact content
	Code int    `json:"code,omitempty"` // exit code
	PID  int    `json:"pid,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Name string `json:"name,omitempty"` // artifact: path relative to the working dir
}

// Package api implements the ansible-ui control-plane HTTP + WebSocket service.
package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nikiv/ansible-ui/internal/config"
	"github.com/nikiv/ansible-ui/internal/crypto"
	"github.com/nikiv/ansible-ui/internal/notify"
	"github.com/nikiv/ansible-ui/internal/store"
	"github.com/nikiv/ansible-ui/internal/webui"
)

// Server wires the store, run manager and HTTP routes together.
type Server struct {
	cfg       config.Config
	store     *store.Store
	log       *slog.Logger
	events    *EventHub
	manager   *Manager
	cipher    *crypto.Cipher
	oidc      *oidcAuth
	saml      *samlSP
	github    *oauthProvider
	bitbucket *oauthProvider
	upgrader  websocket.Upgrader

	// HA dispatch: no in-memory queue/load — the queue lives in Postgres (status
	// queued + an encrypted launch envelope) and only the leader drains it.
	// drainCh nudges the leader's drainer to run a pass (buffered, coalescing).
	drainCh        chan struct{}
	leader         atomic.Bool // true while this replica holds the scheduler/dispatch lock
	wfMu           sync.Mutex  // serialises workflow advancement (parallel steps finish concurrently)
	longRunAlerted sync.Map    // run id → true: a long-running alert already fired (cleared on finish)
}

// NewServer constructs the api server.
func NewServer(cfg config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	cipher, err := crypto.New(cfg.AppSecret)
	if err != nil {
		return nil, err
	}
	events := NewEventHub()
	srv := &Server{
		cfg:     cfg,
		store:   st,
		log:     log,
		events:  events,
		manager: NewManager(st, cfg.RunnerURL, events, log),
		cipher:  cipher,
		drainCh: make(chan struct{}, 1),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 32 * 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}

	// When a run finishes, free its runner slot and dispatch any queued runs.
	srv.manager.onFinish = srv.onRunFinished
	// The runner streams captured artifacts back over the run socket — save them.
	srv.manager.onArtifact = srv.saveArtifact
	// On every run terminal: advance pipelines + forward the event to syslog.
	srv.manager.onTerminal = srv.afterRunTerminal

	// OIDC discovery is best-effort: a missing/unreachable IdP must not stop boot.
	if cfg.OIDC.Enabled && cfg.OIDC.Issuer != "" {
		octx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if o, oerr := newOIDC(octx, cfg.OIDC); oerr != nil {
			log.Warn("oidc init failed; SSO disabled", "err", oerr)
		} else {
			srv.oidc = o
			log.Info("oidc SSO enabled", "issuer", cfg.OIDC.Issuer)
		}
	}
	// SAML SP init is best-effort too (IdP metadata fetch can fail at boot).
	if cfg.SAML.Enabled {
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if sp, serr := newSAML(sctx, cfg.SAML); serr != nil {
			log.Warn("saml init failed; SAML SSO disabled", "err", serr)
		} else {
			srv.saml = sp
			log.Info("saml SSO enabled", "idp", cfg.SAML.IDPMetadataURL)
		}
	}
	if cfg.GitHub.ClientID != "" {
		srv.github = newGitHub(cfg.GitHub, cfg.OAuthRole)
		log.Info("github sign-in enabled")
	}
	if cfg.Bitbucket.ClientID != "" {
		srv.bitbucket = newBitbucket(cfg.Bitbucket, cfg.OAuthRole)
		log.Info("bitbucket sign-in enabled")
	}
	if cfg.LDAP.Enabled {
		log.Info("ldap auth enabled", "url", cfg.LDAP.URL)
	}
	// Register the bundled runner so it appears in the registry (tag "default").
	sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	srv.seedBuiltinRunner(sctx)
	// Apply any configured outbound proxy for notification egress.
	if pu, _, _ := st.GetSetting(sctx, settingNotifyProxy); pu != "" {
		if err := notify.SetProxy(pu); err != nil {
			log.Warn("invalid notify proxy url", "err", err)
		} else {
			log.Info("notification egress via proxy", "proxy", pu)
		}
	}
	return srv, nil
}

// Routes builds the HTTP handler with middleware applied.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/insights", s.handleInsights)
	mux.HandleFunc("GET /metrics", s.handleMetrics) // Prometheus (outside /api → no session guard)
	mux.HandleFunc("POST /api/mcp", s.handleMCP)    // MCP server for AI agents (auth-gated via authGuard)
	mux.HandleFunc("GET /api/settings/retention", s.handleGetRetention)
	mux.HandleFunc("PUT /api/settings/retention", s.handleSetRetention)
	mux.HandleFunc("GET /api/settings/flags", s.handleGetFlags)
	mux.HandleFunc("PUT /api/settings/flags", s.handleSetFlags)
	mux.HandleFunc("GET /api/settings/tasks", s.handleGetTaskSettings)
	mux.HandleFunc("PUT /api/settings/tasks", s.handleSetTaskSettings)
	mux.HandleFunc("GET /api/settings/syslog", s.handleGetSyslog)
	mux.HandleFunc("PUT /api/settings/syslog", s.handleSetSyslog)
	mux.HandleFunc("GET /api/system", s.handleSystemInfo)
	mux.HandleFunc("GET /api/cluster", s.handleCluster)
	mux.HandleFunc("GET /api/audit/export", s.handleExportAudit)
	mux.HandleFunc("GET /api/runs/export", s.handleExportRuns)
	mux.HandleFunc("GET /api/compliance/report", s.handleComplianceReport)

	// auth
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)
	mux.HandleFunc("POST /api/auth/2fa/setup", s.handle2FASetup)
	mux.HandleFunc("POST /api/auth/2fa/enable", s.handle2FAEnable)
	mux.HandleFunc("POST /api/auth/2fa/disable", s.handle2FADisable)
	mux.HandleFunc("POST /api/auth/2fa/email/enable", s.handleEnableEmailOTP)
	mux.HandleFunc("POST /api/auth/2fa/email/disable", s.handleDisableEmailOTP)
	mux.HandleFunc("GET /api/settings/smtp", s.handleGetSMTP)
	mux.HandleFunc("PUT /api/settings/smtp", s.handleSetSMTP)
	mux.HandleFunc("GET /api/settings/galaxy", s.handleGetGalaxy)
	mux.HandleFunc("PUT /api/settings/galaxy", s.handleSetGalaxy)
	mux.HandleFunc("GET /api/settings/notify-proxy", s.handleGetNotifyProxy)
	mux.HandleFunc("PUT /api/settings/notify-proxy", s.handleSetNotifyProxy)
	mux.HandleFunc("GET /api/settings/auth-mapping", s.handleGetAuthMapping)
	mux.HandleFunc("PUT /api/settings/auth-mapping", s.handleSetAuthMapping)
	mux.HandleFunc("GET /api/auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)
	mux.HandleFunc("GET /api/auth/oidc/logout", s.handleOIDCLogout)
	mux.HandleFunc("GET /api/auth/saml/metadata", s.handleSAMLMetadata)
	mux.HandleFunc("GET /api/auth/saml/login", s.handleSAMLLogin)
	mux.HandleFunc("POST /api/auth/saml/acs", s.handleSAMLACS)
	mux.HandleFunc("GET /api/auth/github/login", s.handleGitHubLogin)
	mux.HandleFunc("GET /api/auth/github/callback", s.handleGitHubCallback)
	mux.HandleFunc("GET /api/auth/bitbucket/login", s.handleBitbucketLogin)
	mux.HandleFunc("GET /api/auth/bitbucket/callback", s.handleBitbucketCallback)
	mux.HandleFunc("GET /api/users", s.handleListUsers)
	mux.HandleFunc("POST /api/users", s.handleCreateUser)
	mux.HandleFunc("DELETE /api/users/{id}", s.handleDeleteUser)
	mux.HandleFunc("POST /api/users/{id}/2fa/reset", s.handleAdminReset2FA)
	mux.HandleFunc("GET /api/tokens", s.handleListTokens)
	mux.HandleFunc("POST /api/tokens", s.handleCreateToken)
	mux.HandleFunc("DELETE /api/tokens/{id}", s.handleDeleteToken)

	// key store
	mux.HandleFunc("GET /api/credentials", s.handleListCredentials)
	mux.HandleFunc("POST /api/credentials", s.handleCreateCredential)
	mux.HandleFunc("PUT /api/credentials/{id}", s.handleUpdateCredential)
	mux.HandleFunc("DELETE /api/credentials/{id}", s.handleDeleteCredential)
	mux.HandleFunc("GET /api/credentials/{id}/pubkey", s.handleCredentialPubKey)

	// distributed runners (execution agents with tags/pools)
	mux.HandleFunc("GET /api/runners", s.handleListRunners)
	mux.HandleFunc("POST /api/runners/register", s.handleRegisterRunner)
	mux.HandleFunc("POST /api/runners/{id}/heartbeat", s.handleRunnerHeartbeat)
	mux.HandleFunc("DELETE /api/runners/{id}", s.handleDeleteRunner)

	// external secret managers (HashiCorp Vault, …)
	mux.HandleFunc("GET /api/secret-backends", s.handleListSecretBackends)
	mux.HandleFunc("POST /api/secret-backends", s.handleCreateSecretBackend)
	mux.HandleFunc("POST /api/secret-backends/test", s.handleTestSecretBackend)
	mux.HandleFunc("PUT /api/secret-backends/{id}", s.handleUpdateSecretBackend)
	mux.HandleFunc("DELETE /api/secret-backends/{id}", s.handleDeleteSecretBackend)

	// repositories (git sources)
	mux.HandleFunc("GET /api/repositories", s.handleListRepositories)
	mux.HandleFunc("POST /api/repositories", s.handleCreateRepository)
	mux.HandleFunc("GET /api/repositories/{id}", s.handleGetRepository)
	mux.HandleFunc("PUT /api/repositories/{id}", s.handleUpdateRepository)
	mux.HandleFunc("DELETE /api/repositories/{id}", s.handleDeleteRepository)
	mux.HandleFunc("POST /api/repositories/{id}/sync", s.handleSyncRepository)
	mux.HandleFunc("GET /api/repositories/{id}/tree", s.handleRepoTree)

	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	mux.HandleFunc("PUT /api/projects/{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
	mux.HandleFunc("GET /api/projects/{id}/members", s.handleListMembers)
	mux.HandleFunc("POST /api/projects/{id}/members", s.handleSetMember)
	mux.HandleFunc("DELETE /api/projects/{id}/members/{userId}", s.handleRemoveMember)
	mux.HandleFunc("GET /api/roles", s.handleListCustomRoles)
	mux.HandleFunc("POST /api/roles", s.handleCreateCustomRole)
	mux.HandleFunc("PUT /api/roles/{id}", s.handleUpdateCustomRole)
	mux.HandleFunc("DELETE /api/roles/{id}", s.handleDeleteCustomRole)
	mux.HandleFunc("GET /api/projects/{id}/playbooks", s.handleListPlaybooks)
	mux.HandleFunc("GET /api/projects/{id}/requirements", s.handleProjectRequirements)
	mux.HandleFunc("GET /api/projects/{id}/files", s.handleListFiles)
	mux.HandleFunc("GET /api/projects/{id}/file", s.handleReadFile)
	mux.HandleFunc("PUT /api/projects/{id}/file", s.handleWriteFile)
	mux.HandleFunc("DELETE /api/projects/{id}/file", s.handleDeleteFile)
	mux.HandleFunc("POST /api/projects/{id}/rename", s.handleRenameFile)
	mux.HandleFunc("POST /api/projects/{id}/git-commit", s.handleGitCommit)
	mux.HandleFunc("GET /api/projects/{id}/branches", s.handleListBranches)
	mux.HandleFunc("POST /api/projects/{id}/upload", s.handleUploadArchive)
	mux.HandleFunc("GET /api/projects/{id}/inventories", s.handleListInventories)
	mux.HandleFunc("POST /api/projects/{id}/inventories", s.handleCreateInventory)
	mux.HandleFunc("GET /api/projects/{id}/templates", s.handleProjectTemplates)

	mux.HandleFunc("GET /api/inventories/{id}", s.handleGetInventory)
	mux.HandleFunc("GET /api/inventories/{id}/hosts", s.handleInventoryHosts)
	mux.HandleFunc("POST /api/inventories/{id}/facts/gather", s.handleGatherFacts)
	mux.HandleFunc("POST /api/inventories/{id}/monitor", s.handleEnableMonitor)
	mux.HandleFunc("DELETE /api/inventories/{id}/monitor", s.handleDisableMonitor)
	mux.HandleFunc("GET /api/monitors", s.handleListMonitors)
	mux.HandleFunc("POST /api/monitors/check", s.handleCheckMonitors)
	mux.HandleFunc("GET /api/hosts", s.handleListHosts)
	mux.HandleFunc("GET /api/hosts/{host}", s.handleGetHost)
	mux.HandleFunc("DELETE /api/hosts/{host}", s.handleDeleteHost)
	mux.HandleFunc("PUT /api/inventories/{id}", s.handleUpdateInventory)
	mux.HandleFunc("DELETE /api/inventories/{id}", s.handleDeleteInventory)

	mux.HandleFunc("GET /api/environments", s.handleListEnvironments)
	mux.HandleFunc("POST /api/environments", s.handleCreateEnvironment)
	mux.HandleFunc("GET /api/environments/{id}", s.handleGetEnvironment)
	mux.HandleFunc("PUT /api/environments/{id}", s.handleUpdateEnvironment)
	mux.HandleFunc("DELETE /api/environments/{id}", s.handleDeleteEnvironment)

	mux.HandleFunc("GET /api/templates", s.handleListTemplates)
	mux.HandleFunc("POST /api/templates", s.handleCreateTemplate)
	mux.HandleFunc("GET /api/templates/{id}", s.handleGetTemplate)
	mux.HandleFunc("PUT /api/templates/{id}", s.handleUpdateTemplate)
	mux.HandleFunc("DELETE /api/templates/{id}", s.handleDeleteTemplate)
	mux.HandleFunc("POST /api/templates/{id}/run", s.handleRunTemplate)

	mux.HandleFunc("GET /api/projects/{id}/export", s.handleExportProject)
	mux.HandleFunc("POST /api/projects/import", s.handleImportProject)
	mux.HandleFunc("GET /api/projects/{id}/workflows", s.handleListWorkflows)
	mux.HandleFunc("POST /api/projects/{id}/workflows", s.handleCreateWorkflow)
	mux.HandleFunc("GET /api/workflows/{id}", s.handleGetWorkflow)
	mux.HandleFunc("PUT /api/workflows/{id}", s.handleUpdateWorkflow)
	mux.HandleFunc("DELETE /api/workflows/{id}", s.handleDeleteWorkflow)
	mux.HandleFunc("POST /api/workflows/{id}/run", s.handleRunWorkflow)
	mux.HandleFunc("GET /api/workflows/{id}/versions", s.handleListWorkflowVersions)
	mux.HandleFunc("POST /api/workflows/{id}/rollback/{versionId}", s.handleRollbackWorkflow)
	mux.HandleFunc("GET /api/workflows", s.handleListAllWorkflows)
	mux.HandleFunc("GET /api/templates/all", s.handleListAllTemplates)
	mux.HandleFunc("GET /api/workflow-runs", s.handleListWorkflowRuns)
	mux.HandleFunc("GET /api/workflow-runs/{id}", s.handleGetWorkflowRun)

	mux.HandleFunc("GET /api/views", s.handleListViews)
	mux.HandleFunc("POST /api/views", s.handleCreateView)
	mux.HandleFunc("PUT /api/views/{id}", s.handleUpdateView)
	mux.HandleFunc("DELETE /api/views/{id}", s.handleDeleteView)

	mux.HandleFunc("GET /api/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/schedules", s.handleCreateSchedule)
	mux.HandleFunc("GET /api/schedules/{id}", s.handleGetSchedule)
	mux.HandleFunc("PUT /api/schedules/{id}", s.handleUpdateSchedule)
	mux.HandleFunc("DELETE /api/schedules/{id}", s.handleDeleteSchedule)

	mux.HandleFunc("GET /api/integrations", s.handleListIntegrations)
	mux.HandleFunc("POST /api/integrations", s.handleCreateIntegration)
	mux.HandleFunc("PUT /api/integrations/{id}", s.handleUpdateIntegration)
	mux.HandleFunc("DELETE /api/integrations/{id}", s.handleDeleteIntegration)
	mux.HandleFunc("POST /api/webhooks/{token}", s.handleWebhook) // public (token-authorised)

	mux.HandleFunc("GET /api/activity", s.handleListActivity)

	mux.HandleFunc("GET /api/meta/endpoints", s.handleAPIDocs)
	mux.HandleFunc("PUT /api/meta/docs", s.handleSetDocs)

	mux.HandleFunc("GET /api/applications", s.handleListApplications)
	mux.HandleFunc("POST /api/applications", s.handleCreateApplication)
	mux.HandleFunc("PUT /api/applications/{id}", s.handleUpdateApplication)
	mux.HandleFunc("DELETE /api/applications/{id}", s.handleDeleteApplication)

	mux.HandleFunc("GET /api/notifications", s.handleListNotifications)
	mux.HandleFunc("POST /api/notifications", s.handleCreateNotification)
	mux.HandleFunc("PUT /api/notifications/{id}", s.handleUpdateNotification)
	mux.HandleFunc("DELETE /api/notifications/{id}", s.handleDeleteNotification)
	mux.HandleFunc("POST /api/notifications/{id}/test", s.handleTestNotification)
	mux.HandleFunc("GET /api/notification-logs", s.handleListNotificationLogs)
	mux.HandleFunc("GET /api/secret-access-logs", s.handleListSecretAccessLogs)

	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("POST /api/runs", s.handleCreateRun)
	mux.HandleFunc("DELETE /api/runs", s.handleClearRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/output", s.handleRunOutput)
	mux.HandleFunc("GET /api/runs/{id}/log", s.handleRunLog)
	mux.HandleFunc("GET /api/runs/{id}/secrets", s.handleRunSecrets)
	mux.HandleFunc("GET /api/runs/{id}/artifacts", s.handleListArtifacts)
	mux.HandleFunc("GET /api/runs/{id}/artifacts/{artifactId}", s.handleDownloadArtifact)
	mux.HandleFunc("POST /api/runs/{id}/approve", s.handleApproveRun)
	mux.HandleFunc("POST /api/runs/{id}/reject", s.handleRejectRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancelRun)
	mux.HandleFunc("DELETE /api/runs/{id}", s.handleDeleteRun)

	mux.HandleFunc("GET /ws/events", s.handleEventsWS)
	mux.HandleFunc("GET /ws/runs/{id}", s.handleRunWS)

	// The embedded SPA serves everything that isn't /api or /ws.
	mux.Handle("/", webui.Handler())

	return s.recover(s.cors(s.authGuard(s.logRequests(mux))))
}

// Manager exposes the run manager (used by main for graceful boot tasks).
func (s *Server) Manager() *Manager { return s.manager }

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "api"})
}

// ---- middleware --------------------------------------------------------

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if r.URL.Path != "/api/health" {
			s.log.Debug("http", "method", r.Method, "path", r.URL.Path,
				"status", sw.status, "dur", time.Since(start).String())
		}
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "path", r.URL.Path, "err", rec)
				writeErr(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack delegates to the underlying writer so the WebSocket upgrader keeps
// working through this middleware wrapper.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return h.Hijack()
}

// ---- helpers -----------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// projectScope returns the active project id from the X-Project-Id header. Empty
// means "all projects" (admin/overview); per-project resources use it to scope
// lists to (project_id = scope OR project_id IS NULL) and to stamp new rows.
func projectScope(r *http.Request) string { return r.Header.Get("X-Project-Id") }

// scopePtr turns a scope string into a *string for a project_id column (nil when empty).
func scopePtr(scope string) *string {
	if scope == "" {
		return nil
	}
	return &scope
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) handleStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	s.log.Error("store error", "err", err)
	writeErr(w, http.StatusInternalServerError, "internal error")
}

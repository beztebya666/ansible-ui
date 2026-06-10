// Package config loads service configuration from the environment.
package config

import (
	"os"
	"strings"
)

// Config for the api service.
type Config struct {
	Addr         string // HTTP listen address
	DatabaseURL  string // postgres connection string
	RunnerURL    string // base ws/http URL of the built-in runner service
	RunnerHTTP   string // base http URL of the runner service (git sync)
	RunnerToken  string // shared secret runners present to register/heartbeat (empty = open)
	MetricsToken string // optional bearer for GET /metrics (empty = open, network-restricted)
	DataDir      string // root for project working directories
	SeedDemo     bool   // seed the embedded demo project on first boot
	AppSecret    string // key for encrypting secrets at rest
	SessionTTL   int    // session lifetime in hours
	CookieSecure bool   // set the Secure flag on the session cookie (TLS)
	DocsEnabled  bool   // expose the built-in API Explorer (default true)

	LDAP    LDAPConfig
	OIDC    OIDCConfig
	RADIUS  RADIUSConfig
	TACACS  TACACSConfig
	SAML    SAMLConfig

	GitHub    OAuthConfig
	Bitbucket OAuthConfig
	OAuthRole string // role for users created via GitHub/Bitbucket sign-in
}

// RADIUSConfig configures RADIUS (RFC 2865) authentication (PAP).
type RADIUSConfig struct {
	Enabled     bool
	Server      string // host:port (default port 1812 if none given)
	Secret      string // shared secret between this NAS and the RADIUS server
	NASID       string // NAS-Identifier attribute (optional)
	DefaultRole string
}

// TACACSConfig configures TACACS+ (RFC 8907) authentication (ASCII login).
type TACACSConfig struct {
	Enabled     bool
	Server      string // host:port (default port 49 if none given)
	Secret      string // shared key between this client and the TACACS+ server
	DefaultRole string
}

// SAMLConfig configures SAML 2.0 SSO (SP-initiated, HTTP-Redirect/POST).
type SAMLConfig struct {
	Enabled       bool
	IDPMetadataURL string // URL of the IdP's metadata XML
	RootURL        string // this app's external base URL (for ACS/metadata URLs)
	EntityID       string // SP entity id (default = RootURL + /api/auth/saml/metadata)
	SPCertPEM      string // SP cert (PEM); auto-generated ephemeral if empty
	SPKeyPEM       string // SP private key (PEM); auto-generated ephemeral if empty
	UsernameAttr   string // assertion attribute used as username (default: NameID)
	DefaultRole    string
}

// OAuthConfig configures a plain OAuth2 provider (GitHub / Bitbucket). Enabled
// implicitly when ClientID is set. RedirectURL is optional (providers fall back
// to the app's registered callback when omitted).
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// LDAPConfig configures directory authentication.
type LDAPConfig struct {
	Enabled      bool
	URL          string // ldap://host:389 or ldaps://host:636
	StartTLS     bool
	BindDN       string // service account for search (optional)
	BindPassword string
	UserBaseDN   string // e.g. ou=people,dc=example,dc=org
	UserFilter   string // e.g. (uid=%s) — %s replaced with the username
	EmailAttr    string
	DefaultRole  string
}

// OIDCConfig configures OpenID Connect SSO.
type OIDCConfig struct {
	Enabled       bool
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string // e.g. https://ansible-ui.example/api/auth/oidc/callback
	UsernameClaim string // claim used as username (default: preferred_username)
	DefaultRole   string
}

// Load reads configuration with sensible defaults.
func Load() Config {
	return Config{
		Addr:         env("API_ADDR", ":8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://ansible:ansible@postgres:5432/ansible_ui?sslmode=disable"),
		RunnerURL:    env("RUNNER_URL", "ws://runner:8081"),
		RunnerHTTP:   env("RUNNER_HTTP", "http://runner:8081"),
		RunnerToken:  env("RUNNER_TOKEN", ""),
		MetricsToken: env("METRICS_TOKEN", ""),
		DataDir:      env("DATA_DIR", "/data"),
		SeedDemo:     env("SEED_DEMO", "true") != "false",
		AppSecret:    env("APP_SECRET", "change-me-in-production-please"),
		SessionTTL:   24 * 7,
		CookieSecure: env("COOKIE_SECURE", "false") == "true",
		DocsEnabled:  env("DOCS_ENABLED", "true") != "false",
		LDAP: LDAPConfig{
			Enabled:      env("LDAP_ENABLED", "false") == "true",
			URL:          env("LDAP_URL", ""),
			StartTLS:     env("LDAP_START_TLS", "false") == "true",
			BindDN:       env("LDAP_BIND_DN", ""),
			BindPassword: env("LDAP_BIND_PASSWORD", ""),
			UserBaseDN:   env("LDAP_USER_BASE_DN", ""),
			UserFilter:   env("LDAP_USER_FILTER", "(uid=%s)"),
			EmailAttr:    env("LDAP_EMAIL_ATTR", "mail"),
			DefaultRole:  env("LDAP_DEFAULT_ROLE", "user"),
		},
		OIDC: OIDCConfig{
			Enabled:       env("OIDC_ENABLED", "false") == "true",
			Issuer:        env("OIDC_ISSUER", ""),
			ClientID:      env("OIDC_CLIENT_ID", ""),
			ClientSecret:  env("OIDC_CLIENT_SECRET", ""),
			RedirectURL:   env("OIDC_REDIRECT_URL", ""),
			UsernameClaim: env("OIDC_USERNAME_CLAIM", "preferred_username"),
			DefaultRole:   env("OIDC_DEFAULT_ROLE", "user"),
		},
		RADIUS: RADIUSConfig{
			Enabled:     env("RADIUS_ENABLED", "false") == "true",
			Server:      env("RADIUS_SERVER", ""),
			Secret:      env("RADIUS_SECRET", ""),
			NASID:       env("RADIUS_NAS_ID", "ansible-ui"),
			DefaultRole: env("RADIUS_DEFAULT_ROLE", "user"),
		},
		TACACS: TACACSConfig{
			Enabled:     env("TACACS_ENABLED", "false") == "true",
			Server:      env("TACACS_SERVER", ""),
			Secret:      env("TACACS_SECRET", ""),
			DefaultRole: env("TACACS_DEFAULT_ROLE", "user"),
		},
		SAML: SAMLConfig{
			Enabled:        env("SAML_ENABLED", "false") == "true",
			IDPMetadataURL: env("SAML_IDP_METADATA_URL", ""),
			RootURL:        env("SAML_ROOT_URL", ""),
			EntityID:       env("SAML_ENTITY_ID", ""),
			SPCertPEM:      env("SAML_SP_CERT", ""),
			SPKeyPEM:       env("SAML_SP_KEY", ""),
			UsernameAttr:   env("SAML_USERNAME_ATTR", ""),
			DefaultRole:    env("SAML_DEFAULT_ROLE", "user"),
		},
		GitHub: OAuthConfig{
			ClientID:     env("GITHUB_CLIENT_ID", ""),
			ClientSecret: env("GITHUB_CLIENT_SECRET", ""),
			RedirectURL:  env("GITHUB_REDIRECT_URL", ""),
		},
		Bitbucket: OAuthConfig{
			ClientID:     env("BITBUCKET_CLIENT_ID", ""),
			ClientSecret: env("BITBUCKET_CLIENT_SECRET", ""),
			RedirectURL:  env("BITBUCKET_REDIRECT_URL", ""),
		},
		OAuthRole: env("OAUTH_DEFAULT_ROLE", "user"),
	}
}

func env(k, def string) string {
	// Docker / Kubernetes secrets: <KEY>_FILE points at a mounted secret file whose
	// contents are the value (keeps secrets like APP_SECRET out of the env table).
	if path := os.Getenv(k + "_FILE"); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

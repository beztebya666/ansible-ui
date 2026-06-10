package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// ---- HTTP handlers (admin-managed) ------------------------------------

func (s *Server) handleListSecretBackends(w http.ResponseWriter, r *http.Request) {
	backends, err := s.store.ListSecretBackends(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]*model.SecretBackend, 0, len(backends))
	for _, b := range backends {
		out = append(out, redactBackend(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateSecretBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var b model.SecretBackend
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if msg := validateBackend(&b); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if b.Token != "" {
		blob, err := s.cipher.Encrypt([]byte(b.Token))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		b.TokenBlob = blob
	}
	if err := s.store.CreateSecretBackend(r.Context(), &b); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "secretBackend.created", b.Name, b.Type)
	writeJSON(w, http.StatusCreated, redactBackend(&b))
}

func (s *Server) handleUpdateSecretBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	existing, err := s.store.GetSecretBackend(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var b model.SecretBackend
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	b.ID = existing.ID
	// A new token replaces the stored one; an empty token keeps the existing.
	if b.Token != "" {
		blob, eerr := s.cipher.Encrypt([]byte(b.Token))
		if eerr != nil {
			writeErr(w, http.StatusInternalServerError, eerr.Error())
			return
		}
		b.TokenBlob = blob
	} else {
		b.TokenBlob = existing.TokenBlob
	}
	if err := s.store.UpdateSecretBackend(r.Context(), &b); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "secretBackend.updated", b.Name, b.Type)
	writeJSON(w, http.StatusOK, redactBackend(&b))
}

func (s *Server) handleDeleteSecretBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.store.DeleteSecretBackend(r.Context(), r.PathValue("id")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "secretBackend.deleted", r.PathValue("id"), "")
	w.WriteHeader(http.StatusNoContent)
}

// handleTestSecretBackend verifies connectivity + auth against a backend config.
// The body may be a full backend (to test before saving); when it carries an id
// but no token, the stored token is used.
func (s *Server) handleTestSecretBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var b model.SecretBackend
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	// Testing a stored backend by id: fill any field the client omitted from the
	// saved record (so a bare {"id":…} re-tests the stored config), and decrypt
	// the stored credential when none was re-entered.
	if b.ID != "" {
		if existing, err := s.store.GetSecretBackend(r.Context(), b.ID); err == nil {
			if b.Type == "" {
				b.Type = existing.Type
			}
			if b.Region == "" {
				b.Region = existing.Region
			}
			if b.AccessKeyID == "" {
				b.AccessKeyID = existing.AccessKeyID
			}
			if b.TenantID == "" {
				b.TenantID = existing.TenantID
			}
			if b.Address == "" {
				b.Address = existing.Address
			}
			if b.Mount == "" {
				b.Mount = existing.Mount
			}
			if b.Namespace == "" {
				b.Namespace = existing.Namespace
			}
			if b.Token == "" {
				if tok, derr := s.cipher.Decrypt(existing.TokenBlob); derr == nil {
					b.Token = string(tok)
				}
			}
		}
	}
	var (
		msg string
		err error
	)
	switch b.Type {
	case model.SecretBackendAWS:
		msg, err = s.testAWS(r.Context(), &b)
	case model.SecretBackendAzure:
		msg, err = s.testAzure(r.Context(), &b)
	case model.SecretBackendGCP:
		msg, err = s.testGCP(r.Context(), &b)
	case model.SecretBackendCyberArk:
		msg, err = s.testCyberArk(r.Context(), &b)
	default:
		if b.Address == "" {
			writeErr(w, http.StatusBadRequest, "address is required")
			return
		}
		msg, err = s.testVault(r.Context(), &b)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

// validateBackend checks the per-type required fields; returns "" when valid.
func validateBackend(b *model.SecretBackend) string {
	if b.Name == "" {
		return "name is required"
	}
	switch b.Type {
	case model.SecretBackendAWS:
		if b.Region == "" || b.AccessKeyID == "" {
			return "region and access key id are required for AWS Secrets Manager"
		}
	case model.SecretBackendAzure:
		if b.Address == "" || b.TenantID == "" || b.AccessKeyID == "" {
			return "key vault URL, tenant id and client id are required for Azure Key Vault"
		}
	case model.SecretBackendGCP:
		if b.AccessKeyID == "" {
			return "project id is required for GCP Secret Manager"
		}
	case model.SecretBackendCyberArk:
		if b.Address == "" || b.AccessKeyID == "" {
			return "Conjur URL and login (host/…) are required for CyberArk"
		}
	default: // vault
		if b.Address == "" {
			return "address is required"
		}
	}
	return ""
}

func redactBackend(b *model.SecretBackend) *model.SecretBackend {
	c := *b
	c.Token = ""
	c.TokenBlob = nil
	c.HasToken = len(b.TokenBlob) > 0
	return &c
}

// ---- Vault KV v2 client ------------------------------------------------

func (s *Server) vaultHTTP(b *model.SecretBackend) *http.Client {
	tr := &http.Transport{}
	if b.Insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // operator opt-in for self-signed Vault
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

func vaultURL(b *model.SecretBackend, p string) string {
	return strings.TrimRight(b.Address, "/") + "/v1/" + strings.Trim(p, "/")
}

func (s *Server) vaultDo(ctx context.Context, b *model.SecretBackend, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vaultURL(b, path), nil)
	if err != nil {
		return 0, nil, err
	}
	if b.Token != "" {
		req.Header.Set("X-Vault-Token", b.Token)
	}
	if b.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", b.Namespace)
	}
	resp, err := s.vaultHTTP(b).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, body, nil
}

// testVault validates address + token by looking up the token (or, tokenless,
// the server's health). Returns a short human-readable status on success.
func (s *Server) testVault(ctx context.Context, b *model.SecretBackend) (string, error) {
	if b.Token != "" {
		code, body, err := s.vaultDo(ctx, b, "auth/token/lookup-self")
		if err != nil {
			return "", fmt.Errorf("cannot reach Vault: %w", err)
		}
		if code == http.StatusForbidden || code == http.StatusUnauthorized {
			return "", fmt.Errorf("authentication failed (token rejected)")
		}
		if code != http.StatusOK {
			return "", fmt.Errorf("Vault returned HTTP %d", code)
		}
		var lr struct {
			Data struct {
				DisplayName string `json:"display_name"`
				TTL         int    `json:"ttl"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &lr)
		if lr.Data.DisplayName != "" {
			return fmt.Sprintf("authenticated as %s (ttl %ds)", lr.Data.DisplayName, lr.Data.TTL), nil
		}
		return "authenticated", nil
	}
	code, _, err := s.vaultDo(ctx, b, "sys/health")
	if err != nil {
		return "", fmt.Errorf("cannot reach Vault: %w", err)
	}
	// sys/health returns 200 (active), 429 (standby), 472/473 (DR/perf standby).
	if code == http.StatusOK || code == http.StatusTooManyRequests || code == 472 || code == 473 {
		return "reachable (no token configured)", nil
	}
	return "", fmt.Errorf("Vault returned HTTP %d", code)
}

// resolveExternalSecret fetches one referenced secret's value from its backend.
func (s *Server) resolveExternalSecret(ctx context.Context, ref model.ExternalSecretRef) (string, error) {
	b, err := s.store.GetSecretBackend(ctx, ref.Backend)
	if err != nil {
		return "", fmt.Errorf("secret backend not found")
	}
	if tok, derr := s.cipher.Decrypt(b.TokenBlob); derr == nil {
		b.Token = string(tok)
	}
	switch b.Type {
	case model.SecretBackendVault, "":
		return s.vaultKVRead(ctx, b, ref.Path, ref.Field)
	case model.SecretBackendAWS:
		return s.awsSecretRead(ctx, b, ref.Path, ref.Field)
	case model.SecretBackendAzure:
		return s.azureSecretRead(ctx, b, ref.Path, ref.Field)
	case model.SecretBackendGCP:
		return s.gcpSecretRead(ctx, b, ref.Path, ref.Field)
	case model.SecretBackendCyberArk:
		return s.conjurSecretRead(ctx, b, ref.Path, ref.Field)
	default:
		return "", fmt.Errorf("unsupported backend type %q", b.Type)
	}
}

// vaultKVRead reads mount/data/path and returns field (or the whole JSON map).
func (s *Server) vaultKVRead(ctx context.Context, b *model.SecretBackend, path, field string) (string, error) {
	mount := b.Mount
	if mount == "" {
		mount = "secret"
	}
	code, body, err := s.vaultDo(ctx, b, mount+"/data/"+strings.Trim(path, "/"))
	if err != nil {
		return "", fmt.Errorf("cannot reach Vault: %w", err)
	}
	if code == http.StatusNotFound {
		return "", fmt.Errorf("secret %q not found", path)
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("Vault returned HTTP %d for %q", code, path)
	}
	var kv struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &kv); err != nil {
		return "", fmt.Errorf("unexpected Vault response: %w", err)
	}
	if field == "" {
		raw, _ := json.Marshal(kv.Data.Data)
		return string(raw), nil
	}
	v, ok := kv.Data.Data[field]
	if !ok {
		return "", fmt.Errorf("field %q not present in secret %q", field, path)
	}
	return fmt.Sprintf("%v", v), nil
}

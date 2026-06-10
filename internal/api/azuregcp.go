package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// Azure Key Vault + GCP Secret Manager clients. Both acquire an OAuth2 bearer
// token by hand (no cloud SDK) and read a secret over plain HTTPS — mirroring the
// hand-rolled Vault / AWS-SigV4 backends.

func httpClient() *http.Client { return &http.Client{Timeout: 15 * time.Second} }

// pickField returns field from a JSON value, or the whole value when field == "".
func pickField(value, field, path string) (string, error) {
	if field == "" {
		return value, nil
	}
	var kv map[string]any
	if err := json.Unmarshal([]byte(value), &kv); err != nil {
		return "", fmt.Errorf("secret %q is not JSON; cannot select field %q", path, field)
	}
	v, ok := kv[field]
	if !ok {
		return "", fmt.Errorf("field %q not present in secret %q", field, path)
	}
	return fmt.Sprintf("%v", v), nil
}

// ---- Azure Key Vault ----------------------------------------------------

// azureToken gets an Azure AD token for the Key Vault data plane via the client
// credentials grant (tenant + client id + client secret).
func (s *Server) azureToken(ctx context.Context, b *model.SecretBackend) (string, error) {
	if b.TenantID == "" || b.AccessKeyID == "" {
		return "", fmt.Errorf("tenant id and client id are required")
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {b.AccessKeyID},
		"client_secret": {b.Token},
		"scope":         {"https://vault.azure.net/.default"},
	}
	endpoint := "https://login.microsoftonline.com/" + b.TenantID + "/oauth2/v2.0/token"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Azure AD: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &tr)
	if tr.AccessToken == "" {
		if d := firstLine(tr.ErrorDescription); d != "" {
			return "", fmt.Errorf("Azure AD: %s", d)
		}
		if tr.Error != "" {
			return "", fmt.Errorf("Azure AD: %s", tr.Error)
		}
		return "", fmt.Errorf("Azure AD returned HTTP %d", resp.StatusCode)
	}
	return tr.AccessToken, nil
}

// firstLine trims a multi-line provider error to its first line (Azure AD error
// descriptions are several lines with trace ids).
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func (s *Server) testAzure(ctx context.Context, b *model.SecretBackend) (string, error) {
	if b.Address == "" {
		return "", fmt.Errorf("key vault URL is required")
	}
	if _, err := s.azureToken(ctx, b); err != nil {
		return "", err
	}
	return "authenticated to " + b.Address, nil
}

func (s *Server) azureSecretRead(ctx context.Context, b *model.SecretBackend, path, field string) (string, error) {
	tok, err := s.azureToken(ctx, b)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(b.Address, "/") + "/secrets/" + url.PathEscape(path) + "?api-version=7.4"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Key Vault: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Error.Message != "" {
			return "", fmt.Errorf("Key Vault: %s", e.Error.Message)
		}
		return "", fmt.Errorf("Key Vault returned HTTP %d for %q", resp.StatusCode, path)
	}
	var r struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("unexpected Key Vault response: %w", err)
	}
	return pickField(r.Value, field, path)
}

// ---- GCP Secret Manager -------------------------------------------------

type gcpServiceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// gcpToken signs a service-account JWT (RS256) and exchanges it for an OAuth2
// access token scoped to cloud-platform.
func (s *Server) gcpToken(ctx context.Context, b *model.SecretBackend) (string, error) {
	var sa gcpServiceAccount
	if err := json.Unmarshal([]byte(b.Token), &sa); err != nil || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return "", fmt.Errorf("service account JSON is invalid")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("service account private key is not valid PEM")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("service account private key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service account private key is not RSA")
	}
	now := time.Now()
	enc := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	header := enc(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := enc(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform",
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signing := header + "." + claims
	hashed := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	jwt := signing + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, sa.TokenURI, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Google OAuth: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &tr)
	if tr.AccessToken == "" {
		if tr.ErrorDescription != "" {
			return "", fmt.Errorf("Google OAuth: %s", tr.ErrorDescription)
		}
		if tr.Error != "" {
			return "", fmt.Errorf("Google OAuth: %s", tr.Error)
		}
		return "", fmt.Errorf("Google OAuth returned HTTP %d", resp.StatusCode)
	}
	return tr.AccessToken, nil
}

func (s *Server) testGCP(ctx context.Context, b *model.SecretBackend) (string, error) {
	if b.AccessKeyID == "" {
		return "", fmt.Errorf("project id is required")
	}
	if _, err := s.gcpToken(ctx, b); err != nil {
		return "", err
	}
	return "authenticated (project " + b.AccessKeyID + ")", nil
}

func (s *Server) gcpSecretRead(ctx context.Context, b *model.SecretBackend, path, field string) (string, error) {
	tok, err := s.gcpToken(ctx, b)
	if err != nil {
		return "", err
	}
	// path may be a bare secret name (→ latest) or include "/versions/<v>".
	resource := path
	if !strings.Contains(resource, "/versions/") {
		resource = "secrets/" + resource + "/versions/latest"
	}
	endpoint := "https://secretmanager.googleapis.com/v1/projects/" + url.PathEscape(b.AccessKeyID) + "/" + resource + ":access"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Secret Manager: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Error.Message != "" {
			return "", fmt.Errorf("Secret Manager: %s", e.Error.Message)
		}
		return "", fmt.Errorf("Secret Manager returned HTTP %d for %q", resp.StatusCode, path)
	}
	var r struct {
		Payload struct {
			Data string `json:"data"` // base64
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("unexpected Secret Manager response: %w", err)
	}
	raw, derr := base64.StdEncoding.DecodeString(r.Payload.Data)
	if derr != nil {
		return "", fmt.Errorf("decoding secret payload: %w", derr)
	}
	return pickField(string(raw), field, path)
}

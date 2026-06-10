package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// CyberArk Conjur secret backend (REST, hand-rolled, no SDK). Field reuse on
// SecretBackend: Address = Conjur URL, Namespace = Conjur account, AccessKeyID =
// login (e.g. host/myapp), Token = the API key. A referenced secret's Path is the
// Conjur variable id.

func conjurHTTP(b *model.SecretBackend) *http.Client {
	tr := &http.Transport{}
	if b.Insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // self-signed Conjur appliance
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

func conjurAccount(b *model.SecretBackend) string {
	if a := strings.TrimSpace(b.Namespace); a != "" {
		return a
	}
	return "default"
}

// conjurAuthenticate exchanges the API key for a short-lived access token,
// returned base64-encoded (Accept-Encoding: base64) for the Authorization header.
func conjurAuthenticate(ctx context.Context, b *model.SecretBackend) (string, error) {
	account := conjurAccount(b)
	login := url.PathEscape(strings.TrimSpace(b.AccessKeyID))
	u := strings.TrimRight(b.Address, "/") + "/authn/" + url.PathEscape(account) + "/" + login + "/authenticate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(b.Token))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Encoding", "base64") // → response body is the base64 token
	resp, err := conjurHTTP(b).Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Conjur: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Conjur authn HTTP %d (check account/login/api-key)", resp.StatusCode)
	}
	return strings.TrimSpace(string(body)), nil
}

// conjurSecretRead authenticates then GETs the variable value. field is ignored
// (Conjur variables are opaque values, not JSON maps).
func (s *Server) conjurSecretRead(ctx context.Context, b *model.SecretBackend, path, _ string) (string, error) {
	token, err := conjurAuthenticate(ctx, b)
	if err != nil {
		return "", err
	}
	account := conjurAccount(b)
	u := strings.TrimRight(b.Address, "/") + "/secrets/" + url.PathEscape(account) + "/variable/" + url.PathEscape(strings.Trim(path, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", `Token token="`+token+`"`)
	resp, err := conjurHTTP(b).Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Conjur: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("Conjur variable %q not found", path)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Conjur returned HTTP %d for %q", resp.StatusCode, path)
	}
	return string(body), nil
}

// testCyberArk verifies connectivity + auth by exchanging the API key for a token.
func (s *Server) testCyberArk(ctx context.Context, b *model.SecretBackend) (string, error) {
	if _, err := conjurAuthenticate(ctx, b); err != nil {
		return "", err
	}
	return fmt.Sprintf("authenticated to Conjur (account %q, login %q)", conjurAccount(b), b.AccessKeyID), nil
}

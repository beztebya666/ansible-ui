package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvFileSecret(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "app_secret")
	if err := os.WriteFile(secret, []byte("  s3cr3t-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// <KEY>_FILE wins and is trimmed.
	t.Setenv("APP_SECRET_FILE", secret)
	if got := env("APP_SECRET", "def"); got != "s3cr3t-from-file" {
		t.Fatalf("expected file secret, got %q", got)
	}

	// A missing file falls through to the plain env var.
	t.Setenv("RUNNER_TOKEN_FILE", filepath.Join(dir, "nope"))
	t.Setenv("RUNNER_TOKEN", "plain-token")
	if got := env("RUNNER_TOKEN", "def"); got != "plain-token" {
		t.Fatalf("expected plain env fallback, got %q", got)
	}

	// Neither set → default.
	if got := env("METRICS_TOKEN", "def"); got != "def" {
		t.Fatalf("expected default, got %q", got)
	}
}

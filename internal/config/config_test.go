package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolvePathUsesEnvOverrideAndExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfigPath, "~/secrets/otta.json")

	got := ResolvePath()
	want := filepath.Join(home, "secrets", "otta.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "config.json")

	expires := time.Now().UTC().Add(1 * time.Hour).Round(time.Second)
	original := &Config{
		APIBaseURL: "https://api.moveniumprod.com",
		ClientID:   "ember_app",
		Username:   "nikita",
		Token: Token{
			AccessToken: "token-value",
			TokenType:   "Bearer",
			ExpiresAt:   &expires,
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Username != original.Username {
		t.Fatalf("username mismatch: got %q want %q", loaded.Username, original.Username)
	}
	if strings.TrimSpace(loaded.Token.AccessToken) != "token-value" {
		t.Fatalf("unexpected token value: %q", loaded.Token.AccessToken)
	}
	if loaded.Token.ExpiresAt == nil {
		t.Fatalf("expected expires_at to be present")
	}
}

func TestNormalizeStripsLegacyNilPlaceholders(t *testing.T) {
	cfg := &Config{
		Username: "<nil>",
		Token: Token{
			AccessToken:  "<nil>",
			TokenType:    "<nil>",
			RefreshToken: "<nil>",
			Scope:        "<nil>",
		},
	}

	cfg.Normalize()

	if cfg.Username != "" {
		t.Fatalf("expected empty username, got %q", cfg.Username)
	}
	if cfg.Token.AccessToken != "" || cfg.Token.RefreshToken != "" || cfg.Token.Scope != "" {
		t.Fatalf("expected token placeholders to be cleared, got %#v", cfg.Token)
	}
	if cfg.Token.TokenType != "Bearer" {
		t.Fatalf("expected token type default Bearer, got %q", cfg.Token.TokenType)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := New()
	t.Setenv(EnvAPIBaseURL, "https://example.test")
	t.Setenv(EnvClientID, "docker-client")
	t.Setenv(EnvUsername, "env-user")
	t.Setenv(EnvAccessToken, "env-token")
	t.Setenv(EnvTokenType, "Bearer")
	t.Setenv(EnvRefreshToken, "env-refresh")
	t.Setenv(EnvTokenScope, "all")

	ApplyEnvOverrides(cfg)

	if cfg.APIBaseURL != "https://example.test" {
		t.Fatalf("expected api base override, got %q", cfg.APIBaseURL)
	}
	if cfg.ClientID != "docker-client" {
		t.Fatalf("expected client id override, got %q", cfg.ClientID)
	}
	if cfg.Username != "env-user" {
		t.Fatalf("expected username override, got %q", cfg.Username)
	}
	if cfg.Token.AccessToken != "env-token" {
		t.Fatalf("expected access token override, got %q", cfg.Token.AccessToken)
	}
	if cfg.Token.RefreshToken != "env-refresh" {
		t.Fatalf("expected refresh token override, got %q", cfg.Token.RefreshToken)
	}
	if cfg.Token.Scope != "all" {
		t.Fatalf("expected scope override, got %q", cfg.Token.Scope)
	}
}

package config

import (
	"path/filepath"
	"testing"
)

func TestResolveCachePathUsesEnvOverrideAndExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvCachePath, "~/secrets/otta-cache.json")

	got := ResolveCachePath()
	want := filepath.Join(home, "secrets", "otta-cache.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSaveAndLoadCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "cache.json")
	original := &Cache{
		User: User{
			ID:              24352445,
			Username:        "nikita",
			FirstName:       "Nikita",
			LastName:        "Rabykin",
			Email:           "nikita@example.com",
			Employer:        "Acme",
			WorktimeGroupID: 17910737,
		},
	}

	if err := SaveCache(path, original); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatalf("load cache failed: %v", err)
	}
	if loaded.User.ID != original.User.ID {
		t.Fatalf("expected id %d, got %d", original.User.ID, loaded.User.ID)
	}
	if loaded.User.WorktimeGroupID != original.User.WorktimeGroupID {
		t.Fatalf("expected worktimegroup %d, got %d", original.User.WorktimeGroupID, loaded.User.WorktimeGroupID)
	}
	if loaded.User.FirstName != "Nikita" || loaded.User.LastName != "Rabykin" {
		t.Fatalf("unexpected cached name: %#v %#v", loaded.User.FirstName, loaded.User.LastName)
	}
}

func TestApplyCacheEnvOverrides(t *testing.T) {
	cache := NewCache()
	t.Setenv(EnvUsername, "env-user")
	t.Setenv(EnvUserID, "24352445")
	t.Setenv(EnvWorktimeGroupID, "17910737")

	ApplyCacheEnvOverrides(cache)

	if cache.User.Username != "env-user" {
		t.Fatalf("expected env username, got %q", cache.User.Username)
	}
	if cache.User.ID != 24352445 {
		t.Fatalf("expected env user id, got %d", cache.User.ID)
	}
	if cache.User.WorktimeGroupID != 17910737 {
		t.Fatalf("expected env worktimegroup, got %d", cache.User.WorktimeGroupID)
	}
}

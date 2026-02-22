package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Cache struct {
	User      User      `json:"user"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "~/.otta-cli/cache.json"
	}

	return filepath.Join(home, defaultConfigDir, defaultCache)
}

func ResolveCachePath() string {
	value := strings.TrimSpace(os.Getenv(EnvCachePath))
	if value == "" {
		return DefaultCachePath()
	}

	return expandPath(value)
}

func NewCache() *Cache {
	return &Cache{}
}

func LoadCache(path string) (*Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cache := NewCache()
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, err
	}
	cache.Normalize()

	return cache, nil
}

func SaveCache(path string, cache *Cache) error {
	cache.Normalize()
	cache.UpdatedAt = time.Now().UTC()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o600)
}

func (cache *Cache) Normalize() {
	if cache == nil {
		return
	}

	cache.User.Username = normalizeString(cache.User.Username)
	cache.User.FirstName = normalizeString(cache.User.FirstName)
	cache.User.LastName = normalizeString(cache.User.LastName)
	cache.User.Email = normalizeString(cache.User.Email)
	cache.User.Employer = normalizeString(cache.User.Employer)
}

func ApplyCacheEnvOverrides(cache *Cache) {
	if cache == nil {
		return
	}
	if value := EnvString(EnvUsername); value != "" {
		cache.User.Username = value
	}
	if value, ok := EnvInt64(EnvUserID); ok {
		cache.User.ID = value
	}
	if value, ok := EnvInt64(EnvWorktimeGroupID); ok {
		cache.User.WorktimeGroupID = value
	}
	cache.Normalize()
}

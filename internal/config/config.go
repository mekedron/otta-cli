package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EnvConfigPath      = "OTTA_CLI_CONFIG_PATH"
	EnvCachePath       = "OTTA_CLI_CACHE_PATH"
	EnvAPIBaseURL      = "OTTA_CLI_API_BASE_URL"
	EnvClientID        = "OTTA_CLI_CLIENT_ID"
	EnvUsername        = "OTTA_CLI_USERNAME"
	EnvPassword        = "OTTA_CLI_PASSWORD"
	EnvAccessToken     = "OTTA_CLI_ACCESS_TOKEN"
	EnvTokenType       = "OTTA_CLI_TOKEN_TYPE"
	EnvRefreshToken    = "OTTA_CLI_REFRESH_TOKEN"
	EnvTokenScope      = "OTTA_CLI_TOKEN_SCOPE"
	EnvUserID          = "OTTA_CLI_USER_ID"
	EnvWorktimeGroupID = "OTTA_CLI_WORKTIMEGROUP_ID"
	DefaultAPIBase     = "https://api.moveniumprod.com"
	DefaultClientID    = "ember_app"
	defaultConfigDir   = ".otta-cli"
	defaultConfig      = "config.json"
	defaultCache       = "cache.json"
)

type Token struct {
	AccessToken  string     `json:"access_token"`
	TokenType    string     `json:"token_type,omitempty"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Scope        string     `json:"scope,omitempty"`
}

type User struct {
	ID              int64  `json:"id,omitempty"`
	Username        string `json:"username,omitempty"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	Email           string `json:"email,omitempty"`
	Employer        string `json:"employer,omitempty"`
	WorktimeGroupID int64  `json:"worktimegroup_id,omitempty"`
}

type Config struct {
	APIBaseURL string    `json:"api_base_url"`
	ClientID   string    `json:"client_id"`
	Username   string    `json:"username,omitempty"`
	Token      Token     `json:"token"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "~/.otta-cli/config.json"
	}

	return filepath.Join(home, defaultConfigDir, defaultConfig)
}

func ResolvePath() string {
	value := strings.TrimSpace(os.Getenv(EnvConfigPath))
	if value == "" {
		return DefaultPath()
	}

	return expandPath(value)
}

func New() *Config {
	return &Config{
		APIBaseURL: DefaultAPIBase,
		ClientID:   DefaultClientID,
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := New()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.Normalize()

	return cfg, nil
}

func Save(path string, cfg *Config) error {
	cfg.Normalize()
	cfg.UpdatedAt = time.Now().UTC()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o600)
}

func (cfg *Config) Normalize() {
	if cfg == nil {
		return
	}
	cfg.Username = normalizeString(cfg.Username)
	cfg.Token.AccessToken = normalizeString(cfg.Token.AccessToken)
	cfg.Token.TokenType = normalizeString(cfg.Token.TokenType)
	cfg.Token.RefreshToken = normalizeString(cfg.Token.RefreshToken)
	cfg.Token.Scope = normalizeString(cfg.Token.Scope)

	if strings.TrimSpace(cfg.APIBaseURL) == "" {
		cfg.APIBaseURL = DefaultAPIBase
	}
	cfg.APIBaseURL = strings.TrimSpace(cfg.APIBaseURL)
	if strings.TrimSpace(cfg.ClientID) == "" {
		cfg.ClientID = DefaultClientID
	}
	if strings.TrimSpace(cfg.Token.TokenType) == "" {
		cfg.Token.TokenType = "Bearer"
	}
}

func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if value := EnvString(EnvAPIBaseURL); value != "" {
		cfg.APIBaseURL = value
	}
	if value := EnvString(EnvClientID); value != "" {
		cfg.ClientID = value
	}
	if value := EnvString(EnvUsername); value != "" {
		cfg.Username = value
	}
	if value := EnvString(EnvAccessToken); value != "" {
		cfg.Token.AccessToken = value
	}
	if value := EnvString(EnvTokenType); value != "" {
		cfg.Token.TokenType = value
	}
	if value := EnvString(EnvRefreshToken); value != "" {
		cfg.Token.RefreshToken = value
	}
	if value := EnvString(EnvTokenScope); value != "" {
		cfg.Token.Scope = value
	}
	cfg.Normalize()
}

func EnvInt64(key string) (int64, bool) {
	value := EnvString(key)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func EnvString(key string) string {
	return normalizeString(os.Getenv(key))
}

func normalizeString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "<nil>" {
		return ""
	}
	return trimmed
}

func expandPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, "~") {
		return trimmed
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return trimmed
	}
	if trimmed == "~" {
		return home
	}
	if strings.HasPrefix(trimmed, "~/") {
		return filepath.Join(home, trimmed[2:])
	}

	return trimmed
}

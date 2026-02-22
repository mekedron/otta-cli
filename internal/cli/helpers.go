package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
)

type commandResult struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tokenRefreshLeadTime = 30 * time.Second

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func loadConfigOrNew(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return config.New(), nil
	}
	return nil, err
}

func loadRuntimeConfig(path string) (*config.Config, error) {
	cfg, err := loadConfigOrNew(path)
	if err == nil {
		config.ApplyEnvOverrides(cfg)
		return cfg, nil
	}
	return nil, err
}

func loadCacheOrNew(path string) (*config.Cache, error) {
	cache, err := config.LoadCache(path)
	if err == nil {
		return cache, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return config.NewCache(), nil
	}
	return nil, err
}

func loadRuntimeCache(path string) (*config.Cache, error) {
	cache, err := loadCacheOrNew(path)
	if err == nil {
		config.ApplyCacheEnvOverrides(cache)
		return cache, nil
	}
	return nil, err
}

func newAPIClient(cfg *config.Config, configPath string) *otta.Client {
	client := otta.NewClient(cfg.APIBaseURL, nil)
	client.SetToken(cfg.Token.TokenType, cfg.Token.AccessToken)
	if strings.TrimSpace(cfg.Token.RefreshToken) == "" {
		return client
	}

	client.SetTokenRefreshPolicy(func() bool {
		return tokenExpiresSoon(cfg.Token.ExpiresAt, time.Now().UTC())
	})
	client.SetTokenRefresher(func(ctx context.Context) (*otta.LoginResponse, error) {
		response, err := client.RefreshToken(ctx, cfg.Token.RefreshToken, firstNonEmpty(cfg.ClientID, config.DefaultClientID))
		if err != nil {
			return nil, err
		}
		applyRefreshedToken(cfg, response)
		if shouldPersistTokenRefresh(configPath) {
			if err := config.Save(configPath, cfg); err != nil {
				return nil, err
			}
		}
		return response, nil
	})

	return client
}

func requireAccessToken(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is not initialized")
	}
	if strings.TrimSpace(cfg.Token.AccessToken) == "" {
		return fmt.Errorf("no access token configured (run `otta auth login`)")
	}
	return nil
}

func tokenExpiresSoon(expiresAt *time.Time, now time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return !expiresAt.UTC().After(now.UTC().Add(tokenRefreshLeadTime))
}

func shouldPersistTokenRefresh(configPath string) bool {
	if strings.TrimSpace(configPath) == "" {
		return false
	}
	if _, err := os.Stat(configPath); err != nil {
		return false
	}
	if config.EnvString(config.EnvAccessToken) != "" {
		return false
	}
	if config.EnvString(config.EnvTokenType) != "" {
		return false
	}
	if config.EnvString(config.EnvRefreshToken) != "" {
		return false
	}
	if config.EnvString(config.EnvTokenScope) != "" {
		return false
	}
	return true
}

func applyLoginToken(cfg *config.Config, token *otta.LoginResponse) {
	applyTokenResponse(cfg, token, false)
}

func applyRefreshedToken(cfg *config.Config, token *otta.LoginResponse) {
	applyTokenResponse(cfg, token, true)
}

func applyTokenResponse(cfg *config.Config, token *otta.LoginResponse, preserveExisting bool) {
	if cfg == nil || token == nil {
		return
	}

	cfg.Token.AccessToken = strings.TrimSpace(token.AccessToken)
	cfg.Token.TokenType = firstNonEmpty(token.TokenType, cfg.Token.TokenType, "Bearer")
	if preserveExisting {
		if value := strings.TrimSpace(token.RefreshToken); value != "" {
			cfg.Token.RefreshToken = value
		}
		if value := strings.TrimSpace(token.Scope); value != "" {
			cfg.Token.Scope = value
		}
	} else {
		cfg.Token.RefreshToken = strings.TrimSpace(token.RefreshToken)
		cfg.Token.Scope = strings.TrimSpace(token.Scope)
	}
	if token.ExpiresIn > 0 {
		expiresAt := time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
		cfg.Token.ExpiresAt = &expiresAt
	} else {
		cfg.Token.ExpiresAt = nil
	}

	cfg.Normalize()
}

func userDisplayName(user config.User) string {
	full := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if full != "" {
		return full
	}
	if strings.TrimSpace(user.Username) != "" {
		return strings.TrimSpace(user.Username)
	}
	if strings.TrimSpace(user.Email) != "" {
		return strings.TrimSpace(user.Email)
	}
	return ""
}

func parseISODate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func formatISODate(t time.Time) string {
	return t.Format("2006-01-02")
}

func extractBestUser(raw any) config.User {
	return extractBestUserRecursive(raw)
}

func extractBestUserRecursive(raw any) config.User {
	switch typed := raw.(type) {
	case map[string]any:
		if user, ok := parseUserFromMap(typed); ok {
			return user
		}
		for _, key := range []string{"user", "users", "included", "data", "worktimes", "items", "results"} {
			if value, exists := typed[key]; exists {
				user := extractBestUserRecursive(value)
				if hasUserData(user) {
					return user
				}
			}
		}
		for _, value := range typed {
			user := extractBestUserRecursive(value)
			if hasUserData(user) {
				return user
			}
		}
	case []any:
		for _, value := range typed {
			user := extractBestUserRecursive(value)
			if hasUserData(user) {
				return user
			}
		}
	}

	return config.User{}
}

func parseUserFromMap(item map[string]any) (config.User, bool) {
	user := config.User{
		ID:        getInt64(item, "id"),
		Username:  getString(item, "username"),
		FirstName: firstNonEmpty(getString(item, "firstname"), getString(item, "first_name")),
		LastName:  firstNonEmpty(getString(item, "lastname"), getString(item, "last_name")),
		Email:     getString(item, "email"),
	}

	if employer, ok := item["employer"].(map[string]any); ok {
		user.Employer = firstNonEmpty(getString(employer, "name"), getString(employer, "company_name"))
	}

	worktimeGroupRaw := item["worktimegroup"]
	if worktimeGroupRaw == nil {
		worktimeGroupRaw = item["worktime_group"]
	}
	switch typed := worktimeGroupRaw.(type) {
	case map[string]any:
		user.WorktimeGroupID = getInt64(typed, "id")
	default:
		user.WorktimeGroupID = toInt64(typed)
	}

	if !hasUserData(user) {
		return config.User{}, false
	}

	return user, true
}

func hasUserData(user config.User) bool {
	return user.ID > 0 ||
		strings.TrimSpace(user.Username) != "" ||
		strings.TrimSpace(user.FirstName) != "" ||
		strings.TrimSpace(user.LastName) != "" ||
		strings.TrimSpace(user.Email) != "" ||
		strings.TrimSpace(user.Employer) != "" ||
		user.WorktimeGroupID > 0
}

func getString(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		rendered := strings.TrimSpace(typed)
		if rendered == "<nil>" {
			return ""
		}
		return rendered
	default:
		rendered := strings.TrimSpace(fmt.Sprint(typed))
		if rendered == "<nil>" {
			return ""
		}
		return rendered
	}
}

func getInt64(values map[string]any, key string) int64 {
	return toInt64(values[key])
}

func toInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		if typed > math.MaxInt64 {
			return 0
		}
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		value64, err := typed.Int64()
		if err == nil {
			return value64
		}
		valueFloat, err := typed.Float64()
		if err != nil {
			return 0
		}
		return int64(valueFloat)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return int64(parsed)
		}
		return 0
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
}

func countList(raw any, keys ...string) int {
	switch typed := raw.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		for _, key := range keys {
			if value, ok := typed[key]; ok {
				if list, ok := value.([]any); ok {
					return len(list)
				}
			}
		}
		if value, ok := typed["data"]; ok {
			if list, ok := value.([]any); ok {
				return len(list)
			}
		}
	}
	return 0
}

func enrichUserFromAPI(ctx context.Context, client *otta.Client, cache *config.Cache, usernameHint string) error {
	if cache == nil {
		return nil
	}

	var me map[string]any
	if err := client.Request(ctx, "GET", "/me", nil, nil, &me); err != nil {
		return err
	}

	if userID := toInt64(me["userid"]); userID > 0 {
		cache.User.ID = userID
	}
	cache.User.Username = firstNonEmpty(getString(me, "username"), cache.User.Username, usernameHint)

	if cache.User.ID <= 0 {
		return nil
	}

	var raw any
	if err := client.Request(ctx, "GET", fmt.Sprintf("/users/%d", cache.User.ID), map[string]string{"sideload": "true"}, nil, &raw); err != nil {
		// Keep auth/status usable even if user profile endpoint is temporarily unavailable.
		return nil
	}

	root, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	userMap, ok := root["user"].(map[string]any)
	if !ok {
		return nil
	}
	parsed, ok := parseUserFromMap(userMap)
	if !ok {
		return nil
	}

	mergeUser(cache, parsed)

	return nil
}

func mergeUser(cache *config.Cache, parsed config.User) {
	if cache == nil {
		return
	}

	if parsed.ID > 0 {
		cache.User.ID = parsed.ID
	}
	cache.User.Username = firstNonEmpty(parsed.Username, cache.User.Username)
	cache.User.FirstName = firstNonEmpty(parsed.FirstName, cache.User.FirstName)
	cache.User.LastName = firstNonEmpty(parsed.LastName, cache.User.LastName)
	cache.User.Email = firstNonEmpty(parsed.Email, cache.User.Email)
	cache.User.Employer = firstNonEmpty(parsed.Employer, cache.User.Employer)
	if parsed.WorktimeGroupID > 0 {
		cache.User.WorktimeGroupID = parsed.WorktimeGroupID
	}
}

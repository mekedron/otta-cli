package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
)

type cliJSONResult struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Data    map[string]any `json:"data"`
	Error   string         `json:"error"`
}

func TestAuthLoginCommandE2E(t *testing.T) {
	var loginAuthHeader string
	var meAuthHeader string
	var usersAuthHeader string
	var worktimesAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form failed: %v", err)
			}
			if r.Form.Get("grant_type") != "password" {
				t.Fatalf("grant_type mismatch: %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("username") != "nikita" {
				t.Fatalf("username mismatch: %q", r.Form.Get("username"))
			}
			if r.Form.Get("password") != "secret" {
				t.Fatalf("password mismatch: %q", r.Form.Get("password"))
			}
			if r.Form.Get("client_id") != "ember_app" {
				t.Fatalf("client_id mismatch: %q", r.Form.Get("client_id"))
			}
			loginAuthHeader = r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"access_token":"token-login","token_type":"Bearer","expires_in":3600,"user_id":24352445,"username":"nikita"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/me":
			meAuthHeader = r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"userid":24352445,"username":"nikita"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/users/24352445":
			usersAuthHeader = r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"user":{"id":24352445,"firstname":"Nikita","lastname":"Rabykin","username":"nikita","email":"nikita@example.com","worktimegroup":{"id":17910737},"employer":{"name":"Acme"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesAuthHeader = r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"count":0,"worktimes":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"auth", "login", "--username", "nikita", "--password", "secret", "--json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode auth result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "auth login" {
		t.Fatalf("unexpected auth result: %#v", result)
	}
	if result.Data["verified"] != true {
		t.Fatalf("expected verified=true, got %#v", result.Data["verified"])
	}

	stored, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if stored.Token.AccessToken != "token-login" {
		t.Fatalf("expected token-login, got %q", stored.Token.AccessToken)
	}
	if stored.Username != "nikita" {
		t.Fatalf("expected username nikita, got %q", stored.Username)
	}

	cache, err := config.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("load cache failed: %v", err)
	}
	if cache.User.ID != 24352445 {
		t.Fatalf("expected user id 24352445, got %d", cache.User.ID)
	}
	if cache.User.Username != "nikita" {
		t.Fatalf("expected username nikita in cache, got %q", cache.User.Username)
	}
	if cache.User.FirstName != "Nikita" || cache.User.LastName != "Rabykin" {
		t.Fatalf("expected full name in cache, got %#v %#v", cache.User.FirstName, cache.User.LastName)
	}
	if cache.User.WorktimeGroupID != 17910737 {
		t.Fatalf("expected worktimegroup 17910737 in cache, got %d", cache.User.WorktimeGroupID)
	}
	if loginAuthHeader != "" {
		t.Fatalf("did not expect auth header for /login, got %q", loginAuthHeader)
	}
	for _, got := range []string{meAuthHeader, usersAuthHeader, worktimesAuthHeader} {
		if got != "Bearer token-login" {
			t.Fatalf("expected bearer token-login, got %q", got)
		}
	}
}

func TestAuthLoginCommandUsesEnvCredentialsE2E(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form failed: %v", err)
			}
			if r.Form.Get("username") != "env-user" {
				t.Fatalf("username mismatch: %q", r.Form.Get("username"))
			}
			if r.Form.Get("password") != "env-pass" {
				t.Fatalf("password mismatch: %q", r.Form.Get("password"))
			}
			_, _ = io.WriteString(w, `{"access_token":"token-login","token_type":"Bearer","expires_in":3600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/me":
			_, _ = io.WriteString(w, `{"userid":24352445,"username":"env-user"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/users/24352445":
			_, _ = io.WriteString(w, `{"user":{"id":24352445,"firstname":"Env","lastname":"User","username":"env-user","worktimegroup":{"id":17910737}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			_, _ = io.WriteString(w, `{"count":0,"worktimes":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvUsername, "env-user")
	t.Setenv(config.EnvPassword, "env-pass")

	code, _, errOut := runCLI(t, []string{"auth", "login", "--format", "json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	stored, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if stored.Username != "env-user" {
		t.Fatalf("expected username env-user, got %q", stored.Username)
	}
	if stored.Token.AccessToken != "token-login" {
		t.Fatalf("expected token-login, got %q", stored.Token.AccessToken)
	}
}

func TestStatusCommandE2E(t *testing.T) {
	var meCalled bool
	var usersCalled bool
	var worktimesCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/me":
			meCalled = true
			_, _ = io.WriteString(w, `{"userid":24352445,"username":"nikita"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/users/24352445":
			usersCalled = true
			_, _ = io.WriteString(w, `{"user":{"id":24352445,"firstname":"Nikita","lastname":"Rabykin","username":"nikita","email":"nikita@example.com","worktimegroup":{"id":17910737},"employer":{"name":"Acme"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesCalled = true
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":9001}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-status"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"status", "--format", "json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if !meCalled || !usersCalled || !worktimesCalled {
		t.Fatalf("expected /me /users/:id /worktimes calls, got me=%t users=%t worktimes=%t", meCalled, usersCalled, worktimesCalled)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode status result failed: %v", err)
	}
	if !result.OK || result.Command != "status" {
		t.Fatalf("unexpected status result: %#v", result)
	}
	if toInt64(result.Data["entries_today"]) != 1 {
		t.Fatalf("expected entries_today=1, got %#v", result.Data["entries_today"])
	}
	userMap, ok := result.Data["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %#v", result.Data["user"])
	}
	if getString(userMap, "username") != "nikita" {
		t.Fatalf("expected username nikita, got %#v", userMap["username"])
	}
	if getString(userMap, "first_name") != "Nikita" {
		t.Fatalf("expected first_name Nikita, got %#v", userMap["first_name"])
	}

	cache, err := config.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("load cache failed: %v", err)
	}
	if cache.User.WorktimeGroupID != 17910737 {
		t.Fatalf("expected cached worktimegroup 17910737, got %d", cache.User.WorktimeGroupID)
	}
}

func TestStatusCommandWorksWithEnvCredentialsOnlyE2E(t *testing.T) {
	var authHeaders []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/me":
			_, _ = io.WriteString(w, `{"userid":24352445,"username":"nikita"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/users/24352445":
			_, _ = io.WriteString(w, `{"user":{"id":24352445,"firstname":"Nikita","lastname":"Rabykin","username":"nikita","worktimegroup":{"id":17910737}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			_, _ = io.WriteString(w, `{"count":0,"worktimes":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv(config.EnvConfigPath, filepath.Join(t.TempDir(), "missing-config.json"))
	t.Setenv(config.EnvCachePath, filepath.Join(t.TempDir(), "cache.json"))
	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "token-env-only")
	t.Setenv(config.EnvUsername, "nikita")

	code, out, errOut := runCLI(t, []string{"status", "--format", "json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	for _, got := range authHeaders {
		if got != "Bearer token-env-only" {
			t.Fatalf("expected bearer token-env-only, got %q", got)
		}
	}
	if !strings.Contains(out, `"command": "status"`) {
		t.Fatalf("unexpected status output: %s", out)
	}
}

func TestStatusCommandRefreshesExpiredTokenE2E(t *testing.T) {
	var (
		refreshCalled int
		authHeaders   []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			refreshCalled++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form failed: %v", err)
			}
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Fatalf("grant_type mismatch: %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("refresh_token") != "refresh-old" {
				t.Fatalf("refresh_token mismatch: %q", r.Form.Get("refresh_token"))
			}
			if r.Form.Get("client_id") != "ember_app" {
				t.Fatalf("client_id mismatch: %q", r.Form.Get("client_id"))
			}
			_, _ = io.WriteString(w, `{"access_token":"token-refreshed","refresh_token":"refresh-new","token_type":"Bearer","scope":"all","expires_in":3600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/me":
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"userid":24352445,"username":"nikita"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/users/24352445":
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"user":{"id":24352445,"firstname":"Nikita","lastname":"Rabykin","username":"nikita","worktimegroup":{"id":17910737}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"count":0,"worktimes":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-expired"
	cfg.Token.RefreshToken = "refresh-old"
	expired := time.Now().UTC().Add(-2 * time.Minute)
	cfg.Token.ExpiresAt = &expired
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"status", "--format", "json"})
	if code != 0 {
		t.Fatalf("status failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if !strings.Contains(out, `"command": "status"`) {
		t.Fatalf("unexpected status output: %s", out)
	}
	if refreshCalled != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCalled)
	}
	for _, got := range authHeaders {
		if got != "Bearer token-refreshed" {
			t.Fatalf("expected refreshed auth header, got %q", got)
		}
	}

	stored, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if stored.Token.AccessToken != "token-refreshed" {
		t.Fatalf("expected refreshed access token, got %q", stored.Token.AccessToken)
	}
	if stored.Token.RefreshToken != "refresh-new" {
		t.Fatalf("expected refreshed refresh token, got %q", stored.Token.RefreshToken)
	}
	if stored.Token.ExpiresAt == nil || stored.Token.ExpiresAt.UTC().Before(time.Now().UTC()) {
		t.Fatalf("expected future token expiry, got %#v", stored.Token.ExpiresAt)
	}
}

func TestWorktimesListRefreshesTokenAfterUnauthorizedE2E(t *testing.T) {
	var (
		worktimesCalls int
		refreshCalled  int
		authHeaders    []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			refreshCalled++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form failed: %v", err)
			}
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Fatalf("grant_type mismatch: %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("refresh_token") != "refresh-stale" {
				t.Fatalf("refresh_token mismatch: %q", r.Form.Get("refresh_token"))
			}
			_, _ = io.WriteString(w, `{"access_token":"token-renewed","token_type":"Bearer","expires_in":3600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesCalls++
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			if worktimesCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":"invalid_token","error_description":"expired token"}`)
				return
			}
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":1}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-stale"
	cfg.Token.RefreshToken = "refresh-stale"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"worktimes", "list", "--date", "2026-02-20", "--format", "json"})
	if code != 0 {
		t.Fatalf("worktimes list failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if !strings.Contains(out, `"command": "worktimes list"`) {
		t.Fatalf("unexpected worktimes list output: %s", out)
	}
	if refreshCalled != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCalled)
	}
	if worktimesCalls != 2 {
		t.Fatalf("expected 2 worktimes calls, got %d", worktimesCalls)
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 auth headers, got %#v", authHeaders)
	}
	if authHeaders[0] != "Bearer token-stale" {
		t.Fatalf("expected stale token in first request, got %q", authHeaders[0])
	}
	if authHeaders[1] != "Bearer token-renewed" {
		t.Fatalf("expected renewed token in second request, got %q", authHeaders[1])
	}

	stored, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if stored.Token.AccessToken != "token-renewed" {
		t.Fatalf("expected renewed token in config, got %q", stored.Token.AccessToken)
	}
}

func TestWorktimesCRUDCommandsE2E(t *testing.T) {
	var gotList bool
	var gotAdd bool
	var gotUpdateFetch bool
	var gotUpdate bool
	var gotDelete bool

	var addPayload map[string]any
	var updatePayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes" && r.URL.Query().Get("date") == "2026-02-20":
			gotList = true
			_, _ = io.WriteString(w, `{"count":0,"worktimes":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/worktimes":
			gotAdd = true
			if err := json.NewDecoder(r.Body).Decode(&addPayload); err != nil {
				t.Fatalf("decode add payload failed: %v", err)
			}
			_, _ = io.WriteString(w, `{"worktime":{"id":7,"status":"open"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes" && r.URL.Query().Get("id") == "7":
			gotUpdateFetch = true
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":7,"status":"open","date":"2026-02-20","starttime":"09:00","endtime":"17:00","pause":30,"project":17911009,"user":24352445,"worktype":18423445,"description":"old","row_info":{"id":"7","status":"normal"},"task":null,"subtask":null,"superior":18698856}]}`)
		case r.Method == http.MethodPut && r.URL.Path == "/worktimes/7":
			gotUpdate = true
			if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
				t.Fatalf("decode update payload failed: %v", err)
			}
			_, _ = io.WriteString(w, `{"worktime":{"id":7,"status":"open","endtime":"16:30","description":"updated"}}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/worktimes/7":
			gotDelete = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-worktimes"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, _, errOut := runCLI(t, []string{"worktimes", "list", "--date", "2026-02-20", "--format", "json"})
	if code != 0 {
		t.Fatalf("list failed code=%d stderr=%q", code, errOut)
	}

	code, addOut, errOut := runCLI(t, []string{
		"worktimes", "add",
		"--date", "2026-02-20",
		"--start", "09:00",
		"--end", "17:00",
		"--pause", "30",
		"--project", "17911009",
		"--user", "24352445",
		"--worktype", "18423445",
		"--description", "new",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("add failed code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(addOut, `"command": "worktimes add"`) {
		t.Fatalf("unexpected add output: %s", addOut)
	}

	code, updateOut, errOut := runCLI(t, []string{
		"worktimes", "update",
		"--id", "7",
		"--end", "16:30",
		"--description", "updated",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("update failed code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(updateOut, `"command": "worktimes update"`) {
		t.Fatalf("unexpected update output: %s", updateOut)
	}

	code, _, errOut = runCLI(t, []string{"worktimes", "delete", "--id", "7", "--format", "json"})
	if code != 0 {
		t.Fatalf("delete failed code=%d stderr=%q", code, errOut)
	}

	if !gotList || !gotAdd || !gotUpdateFetch || !gotUpdate || !gotDelete {
		t.Fatalf("expected all CRUD handlers called, got list=%t add=%t fetch=%t update=%t delete=%t", gotList, gotAdd, gotUpdateFetch, gotUpdate, gotDelete)
	}

	addWorktime, ok := addPayload["worktime"].(map[string]any)
	if !ok {
		t.Fatalf("missing add worktime payload: %#v", addPayload)
	}
	if getString(addWorktime, "date") != "2026-02-20" {
		t.Fatalf("expected add date, got %#v", addWorktime["date"])
	}
	if getString(addWorktime, "description") != "new" {
		t.Fatalf("expected add description new, got %#v", addWorktime["description"])
	}

	updateWorktime, ok := updatePayload["worktime"].(map[string]any)
	if !ok {
		t.Fatalf("missing update worktime payload: %#v", updatePayload)
	}
	if getString(updateWorktime, "status") != "open" {
		t.Fatalf("expected status=open in update payload, got %#v", updateWorktime["status"])
	}
	if getString(updateWorktime, "endtime") != "16:30" {
		t.Fatalf("expected endtime 16:30, got %#v", updateWorktime["endtime"])
	}
	if getString(updateWorktime, "description") != "updated" {
		t.Fatalf("expected updated description, got %#v", updateWorktime["description"])
	}
}

func TestWorktimesBrowseCommandRangeE2E(t *testing.T) {
	var requestedDates []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/worktimes" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}

		dateValue := r.URL.Query().Get("date")
		requestedDates = append(requestedDates, dateValue)
		switch dateValue {
		case "2026-02-20":
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":11,"date":"2026-02-20","starttime":"09:00","endtime":"17:00","pause":30,"description":"feature work"}]}`)
		case "2026-02-21":
			_, _ = io.WriteString(w, `{"count":2,"worktimes":[{"id":12,"date":"2026-02-21","starttime":"10:00","endtime":"15:00","pause":15,"description":"support"},{"id":13,"date":"2026-02-21","starttime":"16:00","endtime":"18:00","pause":0,"description":"review"}]}`)
		default:
			t.Fatalf("unexpected date query: %#v", r.URL.Query())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-browse"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"worktimes", "browse",
		"--from", "2026-02-20",
		"--to", "2026-02-21",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("browse failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if len(requestedDates) != 2 || requestedDates[0] != "2026-02-20" || requestedDates[1] != "2026-02-21" {
		t.Fatalf("unexpected requested dates: %#v", requestedDates)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode browse result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "worktimes browse" {
		t.Fatalf("unexpected browse command result: %#v", result)
	}

	if getString(result.Data, "from") != "2026-02-20" {
		t.Fatalf("expected from=2026-02-20, got %#v", result.Data["from"])
	}
	if getString(result.Data, "to") != "2026-02-21" {
		t.Fatalf("expected to=2026-02-21, got %#v", result.Data["to"])
	}
	if toInt64(result.Data["days"]) != 2 {
		t.Fatalf("expected days=2, got %#v", result.Data["days"])
	}
	if toInt64(result.Data["count"]) != 3 {
		t.Fatalf("expected count=3, got %#v", result.Data["count"])
	}
	if toInt64(result.Data["total_minutes"]) != 855 {
		t.Fatalf("expected total_minutes=855, got %#v", result.Data["total_minutes"])
	}

	items, ok := result.Data["items"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("expected 3 items, got %#v", result.Data["items"])
	}
	responses, ok := result.Data["responses"].([]any)
	if !ok || len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %#v", result.Data["responses"])
	}
}

func TestWorktimesReportCSVCommandE2E(t *testing.T) {
	var requestedDates []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/worktimes" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}

		dateValue := r.URL.Query().Get("date")
		requestedDates = append(requestedDates, dateValue)
		switch dateValue {
		case "2026-02-20":
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":101,"date":"2026-02-20","starttime":"09:00","endtime":"17:00","pause":30,"project":17911009,"user":24352445,"worktype":18423445,"description":"feature work"}]}`)
		case "2026-02-21":
			_, _ = io.WriteString(w, `{"count":1,"worktimes":[{"id":102,"date":"2026-02-21","starttime":"09:30","endtime":"12:30","pause":0,"project":17911009,"user":24352445,"worktype":18423445,"description":"support"}]}`)
		default:
			t.Fatalf("unexpected date query: %#v", r.URL.Query())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-report"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"worktimes", "report",
		"--from", "2026-02-20",
		"--to", "2026-02-21",
		"--format", "csv",
	})
	if code != 0 {
		t.Fatalf("report failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if len(requestedDates) != 2 || requestedDates[0] != "2026-02-20" || requestedDates[1] != "2026-02-21" {
		t.Fatalf("unexpected requested dates: %#v", requestedDates)
	}

	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("decode csv failed: %v\noutput=%s", err, out)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 csv rows (header + 2 entries), got %d %#v", len(records), records)
	}

	expectedHeader := []string{
		"date", "id", "starttime", "endtime", "pause", "minutes", "project", "worktype", "task", "subtask", "superior", "user", "status", "description",
	}
	if strings.Join(records[0], ",") != strings.Join(expectedHeader, ",") {
		t.Fatalf("unexpected csv header: %#v", records[0])
	}
	if records[1][0] != "2026-02-20" || records[1][1] != "101" || records[1][5] != "450" {
		t.Fatalf("unexpected first csv row: %#v", records[1])
	}
	if records[2][0] != "2026-02-21" || records[2][1] != "102" || records[2][5] != "180" {
		t.Fatalf("unexpected second csv row: %#v", records[2])
	}
}

func TestWorktimesOptionsCommandE2E(t *testing.T) {
	var projectsQuery url.Values
	var worktypesQuery url.Values
	var tasksQuery url.Values
	var subtasksQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/worktime/projects":
			projectsQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"projects":[{"id":17911009,"text":"ERP"}],"count":1}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktime/worktypes":
			worktypesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"worktypes":[{"id":18423445,"text":"Development"}],"count":1}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktime/tasks":
			tasksQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"tasks":[{"id":999001,"text":"Backend"}],"count":1}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktime/subtasks":
			subtasksQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"subtasks":[{"id":999002,"text":"CLI"}],"count":1}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-options"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"worktimes", "options",
		"--date", "2026-02-20",
		"--project", "17911009",
		"--worktype", "18423445",
		"--task", "999001",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("worktimes options failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if projectsQuery.Get("limit") != "100" || projectsQuery.Get("offset") != "0" {
		t.Fatalf("unexpected projects query: %#v", projectsQuery)
	}
	if worktypesQuery.Get("limit") != "100" || worktypesQuery.Get("offset") != "0" {
		t.Fatalf("unexpected worktypes query: %#v", worktypesQuery)
	}
	if tasksQuery.Get("project") != "17911009" {
		t.Fatalf("expected tasks project filter, got %q", tasksQuery.Get("project"))
	}
	if subtasksQuery.Get("task") != "999001" {
		t.Fatalf("expected subtasks task filter, got %q", subtasksQuery.Get("task"))
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode options result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "worktimes options" {
		t.Fatalf("unexpected command result: %#v", result)
	}

	optionsRoot, ok := result.Data["options"].(map[string]any)
	if !ok {
		t.Fatalf("missing options payload: %#v", result.Data["options"])
	}

	projects, ok := optionsRoot["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("expected one project option, got %#v", optionsRoot["projects"])
	}
	firstProject, ok := projects[0].(map[string]any)
	if !ok || toInt64(firstProject["id"]) != 17911009 {
		t.Fatalf("unexpected project option: %#v", projects[0])
	}

	worktypes, ok := optionsRoot["worktypes"].([]any)
	if !ok || len(worktypes) != 1 {
		t.Fatalf("expected one worktype option, got %#v", optionsRoot["worktypes"])
	}
	firstWorktype, ok := worktypes[0].(map[string]any)
	if !ok || toInt64(firstWorktype["id"]) != 18423445 {
		t.Fatalf("unexpected worktype option: %#v", worktypes[0])
	}

	tasks, ok := optionsRoot["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("expected one task option, got %#v", optionsRoot["tasks"])
	}
	firstTask, ok := tasks[0].(map[string]any)
	if !ok || toInt64(firstTask["id"]) != 999001 {
		t.Fatalf("unexpected task option: %#v", tasks[0])
	}

	subtasks, ok := optionsRoot["subtasks"].([]any)
	if !ok || len(subtasks) != 1 {
		t.Fatalf("expected one subtask option, got %#v", optionsRoot["subtasks"])
	}
	firstSubtask, ok := subtasks[0].(map[string]any)
	if !ok || toInt64(firstSubtask["id"]) != 999002 {
		t.Fatalf("unexpected subtask option: %#v", subtasks[0])
	}

	superiors, ok := optionsRoot["superiors"].([]any)
	if !ok || len(superiors) != 0 {
		t.Fatalf("expected empty superior options, got %#v", optionsRoot["superiors"])
	}
}

func TestWorktimesAddCommandIncludesTaskFieldsE2E(t *testing.T) {
	var addPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/worktimes" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&addPayload); err != nil {
			t.Fatalf("decode add payload failed: %v", err)
		}
		_, _ = io.WriteString(w, `{"worktime":{"id":7,"status":"open"}}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-add-task-fields"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, _, errOut := runCLI(t, []string{
		"worktimes", "add",
		"--date", "2026-02-20",
		"--start", "09:00",
		"--end", "17:00",
		"--pause", "30",
		"--project", "17911009",
		"--user", "24352445",
		"--worktype", "18423445",
		"--task", "999001",
		"--subtask", "999002",
		"--superior", "18698856",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("add failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	addWorktime, ok := addPayload["worktime"].(map[string]any)
	if !ok {
		t.Fatalf("missing add worktime payload: %#v", addPayload)
	}
	if toInt64(addWorktime["task"]) != 999001 {
		t.Fatalf("expected task 999001, got %#v", addWorktime["task"])
	}
	if toInt64(addWorktime["subtask"]) != 999002 {
		t.Fatalf("expected subtask 999002, got %#v", addWorktime["subtask"])
	}
	if toInt64(addWorktime["superior"]) != 18698856 {
		t.Fatalf("expected superior 18698856, got %#v", addWorktime["superior"])
	}
}

func TestHolidaysCommandUsesCacheWorktimegroupFallbackE2E(t *testing.T) {
	var requestedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ttapi/workdayCalendar/workdayDays" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		requestedQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{"count":0,"workdayDays":[]}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-holidays"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cache := config.NewCache()
	cache.User.WorktimeGroupID = 17910737
	if err := config.SaveCache(cachePath, cache); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"holidays",
		"--from", "2026-02-20",
		"--to", "2026-02-20",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("holidays failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if requestedQuery.Get("date") != "2026-02-20_2026-02-20" {
		t.Fatalf("expected date range query, got %q", requestedQuery.Get("date"))
	}
	if requestedQuery.Get("worktimegroup") != "17910737" {
		t.Fatalf("expected worktimegroup fallback, got %q", requestedQuery.Get("worktimegroup"))
	}
	if !strings.Contains(out, `"command": "holidays"`) {
		t.Fatalf("unexpected holidays output: %s", out)
	}
}

func TestSaldoCommandUsesCacheUserFallbackE2E(t *testing.T) {
	var requestedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ttapi/saldo/get_current_saldo" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		requestedQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{"saldo":"1463","from":"2024-09-01","to":"2026-02-21"}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-saldo"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cache := config.NewCache()
	cache.User.ID = 24352445
	if err := config.SaveCache(cachePath, cache); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"saldo", "--format", "json"})
	if code != 0 {
		t.Fatalf("saldo failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if requestedQuery.Get("userid") != "24352445" {
		t.Fatalf("expected userid fallback, got %q", requestedQuery.Get("userid"))
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode saldo result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "saldo" {
		t.Fatalf("unexpected saldo command result: %#v", result)
	}
	if toInt64(result.Data["user_id"]) != 24352445 {
		t.Fatalf("expected user_id=24352445, got %#v", result.Data["user_id"])
	}
	if getString(result.Data, "from") != "2024-09-01" || getString(result.Data, "to") != "2026-02-21" {
		t.Fatalf("unexpected from/to in saldo data: %#v", result.Data)
	}
	if toInt64(result.Data["cumulative_saldo_minutes"]) != 1463 {
		t.Fatalf("expected cumulative_saldo_minutes=1463, got %#v", result.Data["cumulative_saldo_minutes"])
	}

	duration, ok := result.Data["cumulative_saldo_duration"].(map[string]any)
	if !ok {
		t.Fatalf("missing cumulative_saldo_duration payload: %#v", result.Data["cumulative_saldo_duration"])
	}
	if getString(duration, "format") != "minutes" {
		t.Fatalf("expected duration format=minutes, got %#v", duration["format"])
	}
	if toInt64(duration["minutes"]) != 1463 {
		t.Fatalf("expected duration minutes=1463, got %#v", duration["minutes"])
	}
}

func TestSaldoCommandUsesUserFlagE2E(t *testing.T) {
	var requestedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ttapi/saldo/get_current_saldo" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		requestedQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{"saldo":120,"from":"2026-02-01","to":"2026-02-22"}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-saldo-user-flag"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"saldo", "--user", "999000", "--duration-format", "hours"})
	if code != 0 {
		t.Fatalf("saldo failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if requestedQuery.Get("userid") != "999000" {
		t.Fatalf("expected explicit userid=999000, got %q", requestedQuery.Get("userid"))
	}
	if !strings.Contains(out, "cumulative_saldo_minutes: 120") {
		t.Fatalf("expected text saldo minutes output, got %q", out)
	}
	if !strings.Contains(out, "cumulative_saldo_duration: 2.00 hours") {
		t.Fatalf("expected text saldo duration output, got %q", out)
	}
}

func TestSaldoCommandRequiresUserFallbackE2E(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)

	cfg := config.New()
	cfg.Token.AccessToken = "token-saldo-no-user"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, _, errOut := runCLI(t, []string{"saldo"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for missing user fallback, got %d", code)
	}
	if !strings.Contains(errOut, "--user is required") {
		t.Fatalf("expected missing user fallback error, got %q", errOut)
	}
}

func TestAbsenceCommentCommandE2E(t *testing.T) {
	code, out, errOut := runCLI(t, []string{
		"absence", "comment",
		"--type", "sick",
		"--from", "2026-02-20",
		"--to", "2026-02-20",
		"--details", "Flu symptoms",
	})
	if code != 0 {
		t.Fatalf("absence comment failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	expected := "Absence: sick (2026-02-20 - 2026-02-20): Flu symptoms"
	if strings.TrimSpace(out) != expected {
		t.Fatalf("expected %q, got %q", expected, strings.TrimSpace(out))
	}
}

func TestAbsenceReadCommandE2E(t *testing.T) {
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/abcenses/51744722" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		requestedPath = r.URL.Path
		_, _ = io.WriteString(w, `{"abcense":{"id":51744722,"status":"open","user":24352445,"abcensetype":18423477,"startdate":"2026-02-20","starttime":"","enddate":"2026-02-20","endtime":"","dayamount":1,"absence_hours":7.5,"description":"sick leave"}}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-read"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"absence", "read", "--id", "51744722", "--format", "json"})
	if code != 0 {
		t.Fatalf("absence read failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if requestedPath != "/abcenses/51744722" {
		t.Fatalf("unexpected requested path: %q", requestedPath)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence read result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence read" {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if toInt64(result.Data["id"]) != 51744722 {
		t.Fatalf("expected id=51744722, got %#v", result.Data["id"])
	}
	item, ok := result.Data["item"].(map[string]any)
	if !ok {
		t.Fatalf("missing item payload: %#v", result.Data["item"])
	}
	if getString(item, "description") != "sick leave" {
		t.Fatalf("expected description=sick leave, got %#v", item["description"])
	}
	if toInt64(result.Data["total_minutes"]) != 450 {
		t.Fatalf("expected total_minutes=450, got %#v", result.Data["total_minutes"])
	}
}

func TestAbsenceAddCommandUsesCacheUserFallbackE2E(t *testing.T) {
	var addPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/abcenses" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&addPayload); err != nil {
			t.Fatalf("decode absence add payload failed: %v", err)
		}
		_, _ = io.WriteString(w, `{"abcense":{"id":51744799,"status":"open","user":24352445,"abcensetype":18423477,"startdate":"2026-02-20","enddate":"2026-02-20","description":"initial note"}}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-add"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cache := config.NewCache()
	cache.User.ID = 24352445
	if err := config.SaveCache(cachePath, cache); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"absence", "add",
		"--type", "18423477",
		"--from", "2026-02-20",
		"--to", "2026-02-20",
		"--description", "initial note",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("absence add failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	absencePayload, ok := addPayload["abcense"].(map[string]any)
	if !ok {
		t.Fatalf("missing abcense payload: %#v", addPayload)
	}
	if toInt64(absencePayload["user"]) != 24352445 {
		t.Fatalf("expected fallback user=24352445, got %#v", absencePayload["user"])
	}
	if toInt64(absencePayload["abcensetype"]) != 18423477 {
		t.Fatalf("expected type 18423477, got %#v", absencePayload["abcensetype"])
	}
	if getString(absencePayload, "startdate") != "2026-02-20" || getString(absencePayload, "enddate") != "2026-02-20" {
		t.Fatalf("unexpected start/end dates: %#v", absencePayload)
	}
	if getString(absencePayload, "description") != "initial note" {
		t.Fatalf("unexpected description in add payload: %#v", absencePayload["description"])
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence add result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence add" {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if toInt64(result.Data["id"]) != 51744799 {
		t.Fatalf("expected created id=51744799, got %#v", result.Data["id"])
	}
}

func TestAbsenceUpdateCommandE2E(t *testing.T) {
	var getCalled bool
	var updatePayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/abcenses/51744722":
			getCalled = true
			_, _ = io.WriteString(w, `{"abcense":{"id":51744722,"status":"open","user":24352445,"abcensetype":18423477,"startdate":"2026-02-20","starttime":"","enddate":"2026-02-20","endtime":"","dayamount":1,"absence_hours":7.5,"description":"initial note","row_info":{"status":"normal"}}}`)
		case r.Method == http.MethodPut && r.URL.Path == "/abcenses/51744722":
			if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
				t.Fatalf("decode absence update payload failed: %v", err)
			}
			_, _ = io.WriteString(w, `{"abcense":{"id":51744722,"status":"open","user":24352445,"abcensetype":18423477,"startdate":"2026-02-20","starttime":"","enddate":"2026-02-20","endtime":"","dayamount":1,"absence_hours":7.5,"description":"sick leave"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-update"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"absence", "update",
		"--id", "51744722",
		"--description", "sick leave",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("absence update failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if !getCalled {
		t.Fatalf("expected update to fetch existing absence before PUT")
	}

	updated, ok := updatePayload["abcense"].(map[string]any)
	if !ok {
		t.Fatalf("missing abcense payload: %#v", updatePayload)
	}
	if toInt64(updated["id"]) != 51744722 {
		t.Fatalf("expected id=51744722 in update payload, got %#v", updated["id"])
	}
	if toInt64(updated["abcensetype"]) != 18423477 {
		t.Fatalf("expected preserved abcensetype=18423477, got %#v", updated["abcensetype"])
	}
	if getString(updated, "description") != "sick leave" {
		t.Fatalf("expected updated description=sick leave, got %#v", updated["description"])
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence update result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence update" {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if toInt64(result.Data["id"]) != 51744722 {
		t.Fatalf("expected result id=51744722, got %#v", result.Data["id"])
	}
}

func TestAbsenceDeleteCommandE2E(t *testing.T) {
	var deleteCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/abcenses/51744722" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		deleteCalled = true
		_, _ = io.WriteString(w, `{"removed":true}`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-delete"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"absence", "delete", "--id", "51744722", "--format", "json"})
	if code != 0 {
		t.Fatalf("absence delete failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
	if !deleteCalled {
		t.Fatalf("expected DELETE /abcenses/51744722 call")
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence delete result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence delete" {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if toInt64(result.Data["id"]) != 51744722 {
		t.Fatalf("expected deleted id=51744722, got %#v", result.Data["id"])
	}
}

func TestAbsenceOptionsCommandE2E(t *testing.T) {
	var requestedTypesQuery url.Values
	var requestedUsersQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/abcense/abcensetypes":
			requestedTypesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"abcensetypes":[{"id":18423459,"text":"Extra hours"},{"id":19406377,"values":{"name":"Annual holidays"}}],"count":2}`)
		case r.Method == http.MethodGet && r.URL.Path == "/abcense/users":
			requestedUsersQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{"users":[{"id":24352445,"text":"Rabykin Nikita 14"}],"count":1}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-options"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{"absence", "options", "--type", "days", "--format", "json"})
	if code != 0 {
		t.Fatalf("absence options failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if requestedTypesQuery.Get("limit") != "100" || requestedTypesQuery.Get("offset") != "0" {
		t.Fatalf("unexpected type query: %#v", requestedTypesQuery)
	}
	if requestedTypesQuery.Get("type") != "days" {
		t.Fatalf("expected type filter days, got %q", requestedTypesQuery.Get("type"))
	}
	if requestedUsersQuery.Get("limit") != "100" || requestedUsersQuery.Get("offset") != "0" {
		t.Fatalf("unexpected users query: %#v", requestedUsersQuery)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence options result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence options" {
		t.Fatalf("unexpected command result: %#v", result)
	}

	optionsRoot, ok := result.Data["options"].(map[string]any)
	if !ok {
		t.Fatalf("missing options payload: %#v", result.Data["options"])
	}

	types, ok := optionsRoot["types"].([]any)
	if !ok || len(types) != 2 {
		t.Fatalf("expected two absence types, got %#v", optionsRoot["types"])
	}
	firstType, ok := types[0].(map[string]any)
	if !ok || toInt64(firstType["id"]) != 18423459 {
		t.Fatalf("unexpected first absence type option: %#v", types[0])
	}

	users, ok := optionsRoot["users"].([]any)
	if !ok || len(users) != 1 {
		t.Fatalf("expected one user option, got %#v", optionsRoot["users"])
	}
	firstUser, ok := users[0].(map[string]any)
	if !ok || toInt64(firstUser["id"]) != 24352445 {
		t.Fatalf("unexpected first user option: %#v", users[0])
	}
}

func TestAbsenceBrowseCommandRangeE2E(t *testing.T) {
	var requestedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ttapi/absence/split" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		requestedQuery = r.URL.Query()
		_, _ = io.WriteString(w, `[
			{"id":51744207,"date":"2026-02-20","startdate":"2026-02-20","enddate":"2026-02-20","absence_hours":7.5,"abcensetype":{"id":18423477,"name":"Sick leave - own"},"description":"sick"},
			{"id":51744208,"date":"2026-02-22","startdate":"2026-02-22","enddate":"2026-02-22","hours":2,"abcensetype":{"id":18423480,"name":"Personal errand"},"description":"errand"}
		]`)
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvConfigPath, configPath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-absence-browse"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"absence", "browse",
		"--from", "2026-02-20",
		"--to", "2026-02-22",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("absence browse failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if requestedQuery.Get("startdate") != "2026-02-20" || requestedQuery.Get("enddate") != "2026-02-22" {
		t.Fatalf("unexpected start/end date query: %#v", requestedQuery)
	}
	if requestedQuery.Get("order") != "starttime,endtime" {
		t.Fatalf("expected default order, got %q", requestedQuery.Get("order"))
	}
	if requestedQuery.Get("user") != "self" {
		t.Fatalf("expected default user=self, got %q", requestedQuery.Get("user"))
	}
	if requestedQuery.Get("sideload[]") != "abcensetype.name" {
		t.Fatalf("expected sideload[]=abcensetype.name, got %#v", requestedQuery)
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode absence browse result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "absence browse" {
		t.Fatalf("unexpected command result: %#v", result)
	}

	if getString(result.Data, "from") != "2026-02-20" {
		t.Fatalf("expected from=2026-02-20, got %#v", result.Data["from"])
	}
	if getString(result.Data, "to") != "2026-02-22" {
		t.Fatalf("expected to=2026-02-22, got %#v", result.Data["to"])
	}
	if toInt64(result.Data["days"]) != 3 {
		t.Fatalf("expected days=3, got %#v", result.Data["days"])
	}
	if toInt64(result.Data["count"]) != 2 {
		t.Fatalf("expected count=2, got %#v", result.Data["count"])
	}
	if toInt64(result.Data["total_minutes"]) != 570 {
		t.Fatalf("expected total_minutes=570, got %#v", result.Data["total_minutes"])
	}

	items, ok := result.Data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items, got %#v", result.Data["items"])
	}
	responses, ok := result.Data["responses"].([]any)
	if !ok || len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %#v", result.Data["responses"])
	}

	var day21 map[string]any
	for _, raw := range responses {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if getString(entry, "date") == "2026-02-21" {
			day21 = entry
			break
		}
	}
	if day21 == nil {
		t.Fatalf("missing response for 2026-02-21 in %#v", responses)
	}
	if toInt64(day21["count"]) != 0 || toInt64(day21["total_minutes"]) != 0 {
		t.Fatalf("expected empty day for 2026-02-21, got %#v", day21)
	}
}

func TestCalendarOverviewCommandE2E(t *testing.T) {
	var worktimesQuery url.Values
	var absencesQuery url.Values
	var holidaysQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{
				"count": 2,
				"worktimes": [
					{"id":11,"date":"2026-02-19","starttime":"09:00","endtime":"17:00","pause":30,"description":"feature work"},
					{"id":12,"date":"2026-02-21","starttime":"10:00","endtime":"14:00","pause":0,"description":"support"}
				]
			}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ttapi/absence/split":
			absencesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `[
				{"id":51744207,"date":"2026-02-20","startdate":"2026-02-20","enddate":"2026-02-20","absence_hours":7.5,"abcensetype":{"id":18423477,"name":"Sick leave - own"},"description":"sick"}
			]`)
		case r.Method == http.MethodGet && r.URL.Path == "/ttapi/workdayCalendar/workdayDays":
			holidaysQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{
				"count": 1,
				"workdayDays": [
					{"id":9001,"date":"2026-02-22","name":"Sunday"}
				]
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-calendar-overview"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cache := config.NewCache()
	cache.User.WorktimeGroupID = 17910737
	if err := config.SaveCache(cachePath, cache); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"calendar", "overview",
		"--from", "2026-02-19",
		"--to", "2026-02-22",
		"--format", "json",
	})
	if code != 0 {
		t.Fatalf("calendar overview failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if worktimesQuery.Get("date") != "2026-02-19_2026-02-22" {
		t.Fatalf("unexpected worktimes date query: %#v", worktimesQuery)
	}
	if worktimesQuery.Get("user") != "self" {
		t.Fatalf("expected worktimes user=self, got %q", worktimesQuery.Get("user"))
	}
	if worktimesQuery.Get("sideload") != "true" || worktimesQuery.Get("limit") != "500" {
		t.Fatalf("unexpected worktimes query values: %#v", worktimesQuery)
	}

	if absencesQuery.Get("startdate") != "2026-02-19" || absencesQuery.Get("enddate") != "2026-02-22" {
		t.Fatalf("unexpected absence start/end query: %#v", absencesQuery)
	}
	if absencesQuery.Get("user") != "self" {
		t.Fatalf("expected absences user=self, got %q", absencesQuery.Get("user"))
	}
	if absencesQuery.Get("sideload[]") != "abcensetype.name" {
		t.Fatalf("expected sideload[]=abcensetype.name, got %#v", absencesQuery)
	}

	if holidaysQuery.Get("date") != "2026-02-19_2026-02-22" {
		t.Fatalf("unexpected holidays date query: %#v", holidaysQuery)
	}
	if holidaysQuery.Get("worktimegroup") != "17910737" {
		t.Fatalf("expected worktimegroup fallback, got %q", holidaysQuery.Get("worktimegroup"))
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode calendar overview result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "calendar overview" {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if getString(result.Data, "from") != "2026-02-19" || getString(result.Data, "to") != "2026-02-22" {
		t.Fatalf("unexpected from/to: %#v", result.Data)
	}
	if toInt64(result.Data["days"]) != 4 {
		t.Fatalf("expected days=4, got %#v", result.Data["days"])
	}
	if toInt64(result.Data["worktimegroup"]) != 17910737 {
		t.Fatalf("expected worktimegroup=17910737, got %#v", result.Data["worktimegroup"])
	}

	totals, ok := result.Data["totals"].(map[string]any)
	if !ok {
		t.Fatalf("missing totals payload: %#v", result.Data["totals"])
	}
	if toInt64(totals["worktime_count"]) != 2 || toInt64(totals["worktime_minutes"]) != 690 {
		t.Fatalf("unexpected worktime totals: %#v", totals)
	}
	if toInt64(totals["absence_count"]) != 1 || toInt64(totals["absence_minutes"]) != 450 {
		t.Fatalf("unexpected absence totals: %#v", totals)
	}
	if toInt64(totals["holiday_count"]) != 1 {
		t.Fatalf("unexpected holiday_count: %#v", totals["holiday_count"])
	}
	if toInt64(totals["days_with_worktimes"]) != 2 || toInt64(totals["days_with_absences"]) != 1 || toInt64(totals["days_with_holidays"]) != 1 {
		t.Fatalf("unexpected days-with totals: %#v", totals)
	}
	if toInt64(totals["weekend_days"]) != 2 {
		t.Fatalf("expected weekend_days=2, got %#v", totals["weekend_days"])
	}
	if hours, ok := toFloat64(totals["absence_hours"]); !ok || hours != 7.5 {
		t.Fatalf("expected absence_hours=7.5, got %#v", totals["absence_hours"])
	}

	items, ok := result.Data["items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("expected 4 day rows, got %#v", result.Data["items"])
	}

	dayByDate := map[string]map[string]any{}
	for _, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dayByDate[getString(entry, "date")] = entry
	}

	day20, ok := dayByDate["2026-02-20"]
	if !ok {
		t.Fatalf("missing 2026-02-20 in day rows: %#v", dayByDate)
	}
	day20Worktimes, _ := day20["worktimes"].(map[string]any)
	day20Absences, _ := day20["absences"].(map[string]any)
	if toInt64(day20Worktimes["count"]) != 0 || toInt64(day20Absences["count"]) != 1 || toInt64(day20Absences["total_minutes"]) != 450 {
		t.Fatalf("unexpected 2026-02-20 payload: %#v", day20)
	}

	day22, ok := dayByDate["2026-02-22"]
	if !ok {
		t.Fatalf("missing 2026-02-22 in day rows: %#v", dayByDate)
	}
	if weekend, _ := day22["weekend"].(bool); !weekend {
		t.Fatalf("expected 2026-02-22 weekend=true, got %#v", day22["weekend"])
	}
	day22Holidays, _ := day22["holidays"].(map[string]any)
	if toInt64(day22Holidays["count"]) != 1 {
		t.Fatalf("unexpected 2026-02-22 holidays payload: %#v", day22)
	}
}

func TestCalendarDetailedCommandE2E(t *testing.T) {
	var worktimesQuery url.Values
	var absencesQuery url.Values
	var holidaysQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{
				"count": 1,
				"worktimes": [
					{"id":11,"date":"2026-02-19","starttime":"09:00","endtime":"17:00","pause":30,"description":"feature work"}
				]
			}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ttapi/absence/split":
			absencesQuery = r.URL.Query()
			_, _ = io.WriteString(w, `[
				{"id":51744207,"date":"2026-02-20","startdate":"2026-02-20","enddate":"2026-02-20","absence_hours":7.5,"abcensetype":{"id":18423477,"name":"Sick leave - own"},"description":"sick"}
			]`)
		case r.Method == http.MethodGet && r.URL.Path == "/ttapi/workdayCalendar/workdayDays":
			holidaysQuery = r.URL.Query()
			_, _ = io.WriteString(w, `{
				"count": 1,
				"workdayDays": [
					{"id":"11763","date":"2026-02-22","desc":"Sunday Celebration","minutes":"0","midweek_holiday_pay":"1"}
				]
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvConfigPath, configPath)
	t.Setenv(config.EnvCachePath, cachePath)
	cfg := config.New()
	cfg.APIBaseURL = server.URL
	cfg.Token.AccessToken = "token-calendar-detailed"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cache := config.NewCache()
	cache.User.WorktimeGroupID = 17910737
	if err := config.SaveCache(cachePath, cache); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}

	code, out, errOut := runCLI(t, []string{
		"calendar", "detailed",
		"--from", "2026-02-19",
		"--to", "2026-02-22",
		"--format", "json",
		"--duration-format", "hours",
	})
	if code != 0 {
		t.Fatalf("calendar detailed failed code=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(errOut) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}

	if worktimesQuery.Get("date") != "2026-02-19_2026-02-22" {
		t.Fatalf("unexpected worktimes date query: %#v", worktimesQuery)
	}
	if absencesQuery.Get("startdate") != "2026-02-19" || absencesQuery.Get("enddate") != "2026-02-22" {
		t.Fatalf("unexpected absence start/end query: %#v", absencesQuery)
	}
	if holidaysQuery.Get("worktimegroup") != "17910737" {
		t.Fatalf("expected worktimegroup fallback, got %q", holidaysQuery.Get("worktimegroup"))
	}

	var result cliJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode calendar detailed result failed: %v\noutput=%s", err, out)
	}
	if !result.OK || result.Command != "calendar detailed" {
		t.Fatalf("unexpected command result: %#v", result)
	}

	if _, exists := result.Data["worktimegroup"]; exists {
		t.Fatalf("unexpected worktimegroup key in detailed response: %#v", result.Data)
	}
	if toInt64(result.Data["worktime_group_id"]) != 17910737 {
		t.Fatalf("expected worktime_group_id=17910737, got %#v", result.Data["worktime_group_id"])
	}
	if getString(result.Data, "duration_format") != "hours" {
		t.Fatalf("expected duration_format=hours, got %#v", result.Data["duration_format"])
	}

	durations, ok := result.Data["durations"].(map[string]any)
	if !ok {
		t.Fatalf("missing durations payload: %#v", result.Data["durations"])
	}
	worktimeDuration, ok := durations["worktime"].(map[string]any)
	if !ok {
		t.Fatalf("missing worktime duration payload: %#v", durations["worktime"])
	}
	if getString(worktimeDuration, "format") != "hours" {
		t.Fatalf("expected worktime duration format=hours, got %#v", worktimeDuration["format"])
	}
	if hours, ok := toFloat64(worktimeDuration["value"]); !ok || hours != 7.5 {
		t.Fatalf("expected worktime duration value=7.5 hours, got %#v", worktimeDuration["value"])
	}
	absenceDuration, ok := durations["absence"].(map[string]any)
	if !ok {
		t.Fatalf("missing absence duration payload: %#v", durations["absence"])
	}
	if hours, ok := toFloat64(absenceDuration["value"]); !ok || hours != 7.5 {
		t.Fatalf("expected absence duration value=7.5 hours, got %#v", absenceDuration["value"])
	}

	totals, ok := result.Data["totals"].(map[string]any)
	if !ok {
		t.Fatalf("missing totals payload: %#v", result.Data["totals"])
	}
	if toInt64(totals["worktime_minutes"]) != 450 || toInt64(totals["absence_minutes"]) != 450 {
		t.Fatalf("unexpected minutes totals: %#v", totals)
	}
	if toInt64(totals["holiday_count"]) != 1 {
		t.Fatalf("expected holiday_count=1, got %#v", totals["holiday_count"])
	}
	if toInt64(totals["day_off_days"]) != 3 {
		t.Fatalf("expected day_off_days=3, got %#v", totals["day_off_days"])
	}
	if toInt64(totals["celebration_days"]) != 1 || toInt64(totals["celebration_count"]) != 1 {
		t.Fatalf("unexpected celebration totals: %#v", totals)
	}

	items, ok := result.Data["items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("expected 4 day rows, got %#v", result.Data["items"])
	}

	dayByDate := map[string]map[string]any{}
	for _, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dayByDate[getString(entry, "date")] = entry
	}

	day20, ok := dayByDate["2026-02-20"]
	if !ok {
		t.Fatalf("missing 2026-02-20 in day rows: %#v", dayByDate)
	}
	if off, _ := day20["is_day_off"].(bool); !off {
		t.Fatalf("expected 2026-02-20 is_day_off=true, got %#v", day20["is_day_off"])
	}
	reasons20, _ := day20["day_off_reasons"].([]any)
	if len(reasons20) == 0 || getString(map[string]any{"value": reasons20[0]}, "value") == "" {
		t.Fatalf("expected non-empty day_off_reasons for 2026-02-20, got %#v", day20["day_off_reasons"])
	}

	day22, ok := dayByDate["2026-02-22"]
	if !ok {
		t.Fatalf("missing 2026-02-22 in day rows: %#v", dayByDate)
	}
	celebrations22, _ := day22["celebrations"].([]any)
	if len(celebrations22) != 1 {
		t.Fatalf("expected one celebration on 2026-02-22, got %#v", day22["celebrations"])
	}
}

func runCLI(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Execute(context.Background(), args, "test", &out, &errOut)
	return code, out.String(), errOut.String()
}

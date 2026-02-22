package otta

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginSendsExpectedFormFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		if r.Form.Get("grant_type") != "password" {
			t.Fatalf("grant_type mismatch")
		}
		if r.Form.Get("username") != "nikita" {
			t.Fatalf("username mismatch")
		}
		if r.Form.Get("password") != "secret" {
			t.Fatalf("password mismatch")
		}
		if r.Form.Get("client_id") != "ember_app" {
			t.Fatalf("client_id mismatch")
		}
		_, _ = io.WriteString(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	resp, err := client.Login(context.Background(), "nikita", "secret", "ember_app")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if resp.AccessToken != "tok" {
		t.Fatalf("unexpected token: %q", resp.AccessToken)
	}
}

func TestRefreshTokenSendsExpectedFormFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Fatalf("grant_type mismatch: %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("refresh_token") != "refresh-123" {
			t.Fatalf("refresh_token mismatch: %q", r.Form.Get("refresh_token"))
		}
		if r.Form.Get("client_id") != "ember_app" {
			t.Fatalf("client_id mismatch: %q", r.Form.Get("client_id"))
		}
		_, _ = io.WriteString(w, `{"access_token":"tok-new","token_type":"Bearer","expires_in":3600}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	resp, err := client.RefreshToken(context.Background(), "refresh-123", "ember_app")
	if err != nil {
		t.Fatalf("refresh returned error: %v", err)
	}
	if resp.AccessToken != "tok-new" {
		t.Fatalf("unexpected token: %q", resp.AccessToken)
	}
}

func TestRequestSendsBearerJSONAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/worktimes/1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("sideload") != "true" {
			t.Fatalf("expected sideload=true")
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token-123" {
			t.Fatalf("authorization mismatch: %q", auth)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}
		if !strings.Contains(string(bodyBytes), `"description":"updated"`) {
			t.Fatalf("unexpected body: %s", string(bodyBytes))
		}

		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	client.SetAccessToken("token-123")

	payload := map[string]any{
		"worktime": map[string]any{
			"description": "updated",
		},
	}
	var out map[string]any
	err := client.Request(context.Background(), http.MethodPut, "/worktimes/1", map[string]string{"sideload": "true"}, payload, &out)
	if err != nil {
		t.Fatalf("request returned error: %v", err)
	}
	if value, ok := out["ok"].(bool); !ok || !value {
		t.Fatalf("unexpected response payload: %#v", out)
	}
}

func TestRequestRefreshesBeforeCallWhenPolicyRequires(t *testing.T) {
	var (
		authHeaders   []string
		refreshCalled int
		worktimesCall int
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
			if r.Form.Get("refresh_token") != "refresh-xyz" {
				t.Fatalf("refresh_token mismatch: %q", r.Form.Get("refresh_token"))
			}
			_, _ = io.WriteString(w, `{"access_token":"fresh-token","token_type":"Bearer","expires_in":3600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesCall++
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	client.SetAccessToken("stale-token")
	client.SetTokenRefreshPolicy(func() bool { return true })
	client.SetTokenRefresher(func(ctx context.Context) (*LoginResponse, error) {
		return client.RefreshToken(ctx, "refresh-xyz", "ember_app")
	})

	var out map[string]any
	if err := client.Request(context.Background(), http.MethodGet, "/worktimes", nil, nil, &out); err != nil {
		t.Fatalf("request returned error: %v", err)
	}
	if refreshCalled != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCalled)
	}
	if worktimesCall != 1 {
		t.Fatalf("expected 1 worktimes call, got %d", worktimesCall)
	}
	if len(authHeaders) != 1 || authHeaders[0] != "Bearer fresh-token" {
		t.Fatalf("unexpected auth headers: %#v", authHeaders)
	}
}

func TestRequestRefreshesOnUnauthorizedAndRetries(t *testing.T) {
	var (
		authHeaders   []string
		refreshCalled int
		worktimesCall int
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
			if r.Form.Get("refresh_token") != "refresh-123" {
				t.Fatalf("refresh_token mismatch: %q", r.Form.Get("refresh_token"))
			}
			_, _ = io.WriteString(w, `{"access_token":"fresh-token","token_type":"Bearer","expires_in":3600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/worktimes":
			worktimesCall++
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			if worktimesCall == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":"invalid_token","error_description":"expired token"}`)
				return
			}
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	client.SetAccessToken("expired-token")
	client.SetTokenRefresher(func(ctx context.Context) (*LoginResponse, error) {
		return client.RefreshToken(ctx, "refresh-123", "ember_app")
	})

	var out map[string]any
	if err := client.Request(context.Background(), http.MethodGet, "/worktimes", nil, nil, &out); err != nil {
		t.Fatalf("request returned error: %v", err)
	}
	if refreshCalled != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCalled)
	}
	if worktimesCall != 2 {
		t.Fatalf("expected 2 worktimes calls, got %d", worktimesCall)
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 auth headers, got %#v", authHeaders)
	}
	if authHeaders[0] != "Bearer expired-token" {
		t.Fatalf("unexpected first auth header: %q", authHeaders[0])
	}
	if authHeaders[1] != "Bearer fresh-token" {
		t.Fatalf("unexpected second auth header: %q", authHeaders[1])
	}
}

func TestRequestParsesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_token","error_description":"The access token provided is invalid"}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, server.Client())
	var out any
	err := client.Request(context.Background(), http.MethodGet, "/worktimes", nil, nil, &out)
	if err == nil {
		t.Fatalf("expected error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status mismatch: %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Message, "invalid") {
		t.Fatalf("unexpected message: %q", apiErr.Message)
	}
}

func TestParseErrorMessageHandlesUnknownPayload(t *testing.T) {
	value := parseErrorMessage([]byte(`{"no_message":"x"}`))
	if value != "" {
		t.Fatalf("expected empty message, got %q", value)
	}

	value = parseErrorMessage([]byte(`{"message":"boom"}`))
	if value != "boom" {
		t.Fatalf("expected boom, got %q", value)
	}
	if parseErrorMessage([]byte(`{"error_description":"token expired"}`)) != "token expired" {
		t.Fatalf("expected token expired")
	}
}

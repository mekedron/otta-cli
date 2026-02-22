package otta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	baseURL            string
	httpClient         *http.Client
	token              string
	tokenType          string
	tokenRefresher     TokenRefreshFunc
	tokenRefreshPolicy TokenRefreshPolicy
}

type TokenRefreshFunc func(ctx context.Context) (*LoginResponse, error)

type TokenRefreshPolicy func() bool

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("request failed (%d): %s", e.StatusCode, e.Message)
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: client,
		tokenType:  "Bearer",
	}
}

func (c *Client) SetAccessToken(token string) {
	c.SetToken("Bearer", token)
}

func (c *Client) SetToken(tokenType string, token string) {
	c.token = strings.TrimSpace(token)
	c.tokenType = firstNonEmpty(tokenType, "Bearer")
}

func (c *Client) SetTokenRefresher(refreshFn TokenRefreshFunc) {
	c.tokenRefresher = refreshFn
}

func (c *Client) SetTokenRefreshPolicy(policy TokenRefreshPolicy) {
	c.tokenRefreshPolicy = policy
}

func (c *Client) Login(ctx context.Context, username string, password string, clientID string) (*LoginResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "password")
	values.Set("username", username)
	values.Set("password", password)
	values.Set("client_id", clientID)

	return c.tokenGrant(ctx, values)
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string, clientID string) (*LoginResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("client_id", clientID)

	return c.tokenGrant(ctx, values)
}

func (c *Client) tokenGrant(ctx context.Context, values url.Values) (*LoginResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urlFor("/login", nil), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, apiError(resp.StatusCode, body)
	}

	var payload LoginResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (c *Client) Request(ctx context.Context, method string, endpoint string, query map[string]string, body any, out any) error {
	if c.shouldRefreshToken() {
		if err := c.refreshAccessToken(ctx); err != nil {
			return err
		}
	}

	data, statusCode, err := c.requestRaw(ctx, method, endpoint, query, body)
	if err != nil {
		return err
	}

	if statusCode == http.StatusUnauthorized && c.canRefreshToken() {
		if err := c.refreshAccessToken(ctx); err != nil {
			return err
		}
		data, statusCode, err = c.requestRaw(ctx, method, endpoint, query, body)
		if err != nil {
			return err
		}
	}

	if statusCode >= 400 {
		return apiError(statusCode, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	return json.Unmarshal(data, out)
}

func (c *Client) requestRaw(ctx context.Context, method string, endpoint string, query map[string]string, body any) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.urlFor(endpoint, query), bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", c.authorizationHeader())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return data, resp.StatusCode, nil
}

func (c *Client) canRefreshToken() bool {
	return c != nil && c.tokenRefresher != nil
}

func (c *Client) shouldRefreshToken() bool {
	if !c.canRefreshToken() || c.tokenRefreshPolicy == nil {
		return false
	}
	return c.tokenRefreshPolicy()
}

func (c *Client) refreshAccessToken(ctx context.Context) error {
	if !c.canRefreshToken() {
		return nil
	}
	refreshed, err := c.tokenRefresher(ctx)
	if err != nil {
		return fmt.Errorf("refresh access token: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("refresh access token: empty response")
	}
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return fmt.Errorf("refresh access token: empty access_token in response")
	}
	c.SetToken(firstNonEmpty(refreshed.TokenType, c.tokenType, "Bearer"), refreshed.AccessToken)
	return nil
}

func (c *Client) authorizationHeader() string {
	tokenType := firstNonEmpty(c.tokenType, "Bearer")
	return tokenType + " " + strings.TrimSpace(c.token)
}

func (c *Client) urlFor(endpoint string, query map[string]string) string {
	base, _ := url.Parse(c.baseURL)
	if base == nil {
		base = &url.URL{}
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		withQuery, _ := url.Parse(endpoint)
		if withQuery != nil {
			queryValues := withQuery.Query()
			for key, value := range query {
				if strings.TrimSpace(key) == "" {
					continue
				}
				queryValues.Set(key, value)
			}
			withQuery.RawQuery = queryValues.Encode()
			return withQuery.String()
		}
	}

	cleanPath := endpoint
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if base.Path == "" || base.Path == "/" {
		base.Path = cleanPath
	} else {
		base.Path = path.Join(base.Path, cleanPath)
	}

	queryValues := base.Query()
	for key, value := range query {
		if strings.TrimSpace(key) == "" {
			continue
		}
		queryValues.Set(key, value)
	}
	base.RawQuery = queryValues.Encode()

	return base.String()
}

func apiError(status int, payload []byte) error {
	message := strings.TrimSpace(parseErrorMessage(payload))
	if message == "" {
		message = strings.TrimSpace(string(payload))
	}

	return &APIError{
		StatusCode: status,
		Message:    message,
	}
}

func parseErrorMessage(payload []byte) string {
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return ""
	}

	for _, key := range []string{"message", "error_description", "error"} {
		value := strings.TrimSpace(fmt.Sprint(parsed[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}

	return ""
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

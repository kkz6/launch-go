package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// APIClient is the HTTP client for the Launch API
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	teamID     string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetTeam sets the team context for API requests
func (c *APIClient) SetTeam(teamID string) {
	c.teamID = teamID
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

// Do performs an authenticated API request using the stored PAT
func (c *APIClient) Do(ctx context.Context, method, path string, body any) (*APIResponse, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil || creds.Token == "" {
		return nil, fmt.Errorf("not authenticated. Run 'launchctl auth login' first")
	}

	return c.doRequest(ctx, method, path, creds.Token, body)
}

func (c *APIClient) doRequest(ctx context.Context, method, path, token string, body any) (*APIResponse, error) {
	u, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.teamID != "" {
		req.Header.Set("X-Team-ID", c.teamID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response (status %d): %s", resp.StatusCode, string(respBody))
	}

	if resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("%s", apiResp.Message)
	}

	return &apiResp, nil
}

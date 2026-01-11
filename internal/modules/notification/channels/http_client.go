package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultHTTPClient is the default HTTP client implementation
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient creates a new default HTTP client
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Post makes a POST request to the specified URL
func (c *DefaultHTTPClient) Post(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// Get makes a GET request to the specified URL
func (c *DefaultHTTPClient) Get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	PostFunc func(ctx context.Context, url string, body interface{}) ([]byte, int, error)
	GetFunc  func(ctx context.Context, url string) ([]byte, int, error)
}

// Post calls the mock PostFunc
func (c *MockHTTPClient) Post(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
	if c.PostFunc != nil {
		return c.PostFunc(ctx, url, body)
	}
	return nil, 200, nil
}

// Get calls the mock GetFunc
func (c *MockHTTPClient) Get(ctx context.Context, url string) ([]byte, int, error) {
	if c.GetFunc != nil {
		return c.GetFunc(ctx, url)
	}
	return nil, 200, nil
}

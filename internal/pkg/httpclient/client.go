// Package httpclient provides a unified HTTP client base for all API providers.
// It consolidates common HTTP client patterns used across cloud providers
// (DigitalOcean, Hetzner, Vultr, etc.) and git providers (GitHub, GitLab, Bitbucket).
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Default configuration values
const (
	DefaultTimeout = 30 * time.Second
)

var (
	defaultClient     *http.Client
	defaultClientOnce sync.Once
)

// Default returns a shared HTTP client with sensible defaults.
// This client is safe for concurrent use and should be reused across
// the application to take advantage of connection pooling.
func Default() *http.Client {
	defaultClientOnce.Do(func() {
		defaultClient = &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return defaultClient
}

// WithCustomTimeout creates a new HTTP client with the specified timeout.
// Use this when you need a different timeout than the default 30 seconds.
// Note: This creates a new client instance, so use sparingly.
func WithCustomTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// Client is a configurable HTTP client for making API requests.
type Client struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
	authToken  string
	authScheme string
	retryOpts  *RetryOptions
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// NewClient creates a new HTTP client with the given base URL and options.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		headers:    make(map[string]string),
		authScheme: "Bearer",
	}

	// Set default headers
	c.headers["Content-Type"] = "application/json"

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithAuth sets the authentication token with the default Bearer scheme.
func WithAuth(token string) Option {
	return func(c *Client) {
		c.authToken = token
	}
}

// WithAuthScheme sets the authentication scheme (e.g., "Bearer", "token", "Basic").
func WithAuthScheme(scheme string) Option {
	return func(c *Client) {
		c.authScheme = scheme
	}
}

// WithHeader sets a custom header.
func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.headers[key] = value
	}
}

// WithHeaders sets multiple custom headers.
func WithHeaders(headers map[string]string) Option {
	return func(c *Client) {
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithRetry enables retry logic with the given options.
func WithRetry(opts RetryOptions) Option {
	return func(c *Client) {
		c.retryOpts = &opts
	}
}

// SetAuth sets the authentication token dynamically.
func (c *Client) SetAuth(token string) *Client {
	c.authToken = token
	return c
}

// SetHeader sets a header dynamically.
func (c *Client) SetHeader(key, value string) *Client {
	c.headers[key] = value
	return c
}

// BaseURL returns the base URL of the client.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	return c.Do(ctx, http.MethodGet, path, nil, result)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.Do(ctx, http.MethodPost, path, body, result)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.Do(ctx, http.MethodPut, path, body, result)
}

// Patch performs a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.Do(ctx, http.MethodPatch, path, body, result)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result interface{}) error {
	return c.Do(ctx, http.MethodDelete, path, nil, result)
}

// Do performs an HTTP request with the given method, path, body, and result.
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	req, err := c.NewRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	return c.DoRequest(req, result)
}

// NewRequest creates a new HTTP request with configured headers.
func (c *Client) NewRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = &bytesReader{data: jsonBody}
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Set authentication
	if c.authToken != "" {
		req.Header.Set("Authorization", c.authScheme+" "+c.authToken)
	}

	return req, nil
}

// DoRequest executes an HTTP request and decodes the response.
func (c *Client) DoRequest(req *http.Request, result interface{}) error {
	var resp *http.Response
	var err error

	if c.retryOpts != nil {
		resp, err = c.doWithRetry(req)
	} else {
		resp, err = c.httpClient.Do(req)
	}

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(respBody),
		}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// DoRaw performs a request and returns the raw response.
// The caller is responsible for closing the response body.
func (c *Client) DoRaw(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	req, err := c.NewRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	if c.retryOpts != nil {
		resp, err = c.doWithRetry(req)
	} else {
		resp, err = c.httpClient.Do(req)
	}

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d %s: %s", e.StatusCode, e.Status, e.Body)
}

// IsNotFound returns true if the error is a 404 Not Found error.
func (e *HTTPError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsForbidden returns true if the error is a 403 Forbidden error.
func (e *HTTPError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsUnauthorized returns true if the error is a 401 Unauthorized error.
func (e *HTTPError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsRateLimited returns true if the error is a 429 Too Many Requests error.
func (e *HTTPError) IsRateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// IsServerError returns true if the error is a 5xx server error.
func (e *HTTPError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsHTTPError checks if an error is an HTTPError.
func IsHTTPError(err error) (*HTTPError, bool) {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr, true
	}
	return nil, false
}

// bytesReader is a simple io.Reader implementation for byte slices.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

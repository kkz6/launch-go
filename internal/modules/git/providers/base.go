package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// BaseGitProvider provides common functionality for all Git providers.
// It wraps the httpclient.Client with Git provider-specific helpers.
type BaseGitProvider struct {
	client        *httpclient.Client
	config        *ProviderConfig
	sourceControl *SourceControlData
	providerType  GitProviderType
	baseURL       string
	apiURL        string
}

// BaseGitProviderOption is a functional option for configuring BaseGitProvider.
type BaseGitProviderOption func(*BaseGitProvider)

// WithProviderType sets the provider type.
func WithProviderType(t GitProviderType) BaseGitProviderOption {
	return func(b *BaseGitProvider) {
		b.providerType = t
	}
}

// WithBaseURL sets the base URL (e.g., https://github.com).
func WithBaseURL(url string) BaseGitProviderOption {
	return func(b *BaseGitProvider) {
		b.baseURL = url
	}
}

// WithAPIURL sets the API URL (e.g., https://api.github.com).
func WithAPIURL(url string) BaseGitProviderOption {
	return func(b *BaseGitProvider) {
		b.apiURL = url
	}
}

// NewBaseGitProvider creates a new base Git provider with the given configuration.
func NewBaseGitProvider(config *ProviderConfig, opts ...BaseGitProviderOption) *BaseGitProvider {
	b := &BaseGitProvider{
		config: config,
	}

	for _, opt := range opts {
		opt(b)
	}

	// Create the underlying HTTP client with the API URL
	b.client = httpclient.NewClient(
		b.apiURL,
		httpclient.WithHeader("Accept", "application/json"),
	)

	return b
}

// Config returns the provider configuration.
func (b *BaseGitProvider) Config() *ProviderConfig {
	return b.config
}

// SourceControl returns the source control context.
func (b *BaseGitProvider) SourceControl() *SourceControlData {
	return b.sourceControl
}

// SetSourceControl sets the source control context.
func (b *BaseGitProvider) SetSourceControl(sc *SourceControlData) {
	b.sourceControl = sc
}

// GetType returns the provider type.
func (b *BaseGitProvider) GetType() GitProviderType {
	return b.providerType
}

// APIClient returns the underlying HTTP client for direct API access.
func (b *BaseGitProvider) APIClient() *httpclient.Client {
	return b.client
}

// BaseURL returns the base URL of the provider.
func (b *BaseGitProvider) BaseURL() string {
	return b.baseURL
}

// APIURL returns the API URL of the provider.
func (b *BaseGitProvider) APIURL() string {
	return b.apiURL
}

// GetSSHURL returns the SSH URL for a repository.
func (b *BaseGitProvider) GetSSHURL(repo string) string {
	return b.providerType.SSHURL(repo)
}

// GetHTTPSURL returns the HTTPS URL for a repository.
func (b *BaseGitProvider) GetHTTPSURL(repo string) string {
	return b.providerType.HTTPSURL(repo)
}

// Request creates a new request builder with authentication.
func (b *BaseGitProvider) Request(method, path string, token string) *httpclient.Request {
	req := b.client.Request(method, path)
	if token != "" {
		req.WithHeader("Authorization", "Bearer "+token)
	}
	return req
}

// Get performs an authenticated GET request.
func (b *BaseGitProvider) Get(ctx context.Context, path string, token string, result interface{}) error {
	return b.Request(http.MethodGet, path, token).Do(ctx, result)
}

// Post performs an authenticated POST request.
func (b *BaseGitProvider) Post(ctx context.Context, path string, token string, body interface{}, result interface{}) error {
	return b.Request(http.MethodPost, path, token).
		WithJSONBody(body).
		Do(ctx, result)
}

// Put performs an authenticated PUT request.
func (b *BaseGitProvider) Put(ctx context.Context, path string, token string, body interface{}, result interface{}) error {
	return b.Request(http.MethodPut, path, token).
		WithJSONBody(body).
		Do(ctx, result)
}

// Delete performs an authenticated DELETE request.
func (b *BaseGitProvider) Delete(ctx context.Context, path string, token string, result interface{}) error {
	return b.Request(http.MethodDelete, path, token).Do(ctx, result)
}

// DoRaw performs an authenticated request and returns the raw fiberctx.
// Caller is responsible for closing the response body.
func (b *BaseGitProvider) DoRaw(ctx context.Context, method, path string, token string, body interface{}) (*http.Response, error) {
	req := b.Request(method, path, token)
	if body != nil {
		req.WithJSONBody(body)
	}
	return req.DoRaw(ctx)
}

// Webhook Signature Verification

// VerifyHMACSHA256Signature verifies a webhook signature using HMAC-SHA256.
// This is used by GitHub and Bitbucket.
// The prefix parameter allows for provider-specific prefixes (e.g., "sha256=" for GitHub).
func (b *BaseGitProvider) VerifyHMACSHA256Signature(payload []byte, sig, prefix string) bool {
	if b.config.WebhookSecret == "" {
		return false
	}

	if prefix != "" {
		// Prefixed format (e.g., "sha256=...")
		return security.GitHubSignature.Verify(payload, sig, b.config.WebhookSecret)
	}

	// Raw format (no prefix)
	return security.GitLabSignature.Verify(payload, sig, b.config.WebhookSecret)
}

// VerifyTokenSignature verifies a webhook signature using constant-time comparison.
// This is used by GitLab where the webhook secret is passed as a header token.
func (b *BaseGitProvider) VerifyTokenSignature(token string) bool {
	if b.config.WebhookSecret == "" {
		return false
	}

	return security.SecureCompare(token, b.config.WebhookSecret)
}

// Pagination Helpers

// PaginatedResponse represents a paginated API fiberctx.
type PaginatedResponse struct {
	Items      []map[string]interface{}
	NextPage   string // URL for the next page (used by Bitbucket)
	HasMore    bool   // Whether there are more pages
	TotalCount int    // Total count (if available)
}

// ParseLinkHeader parses GitHub-style Link headers to extract pagination URLs.
// Returns the next page URL if present, empty string otherwise.
func ParseLinkHeader(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(link, ";")
		if len(parts) != 2 {
			continue
		}

		if strings.TrimSpace(parts[1]) == `rel="next"` {
			url := strings.TrimSpace(parts[0])
			return strings.Trim(url, "<>")
		}
	}

	return ""
}

// HasNextPage checks if a Link header indicates there's a next page.
func HasNextPage(linkHeader string) bool {
	return strings.Contains(linkHeader, `rel="next"`)
}

// FetchAllPages fetches all pages of a paginated endpoint.
// The fetchPage function should return the items from a single page and the next page URL.
func (b *BaseGitProvider) FetchAllPages(
	ctx context.Context,
	initialPath string,
	token string,
	extractItems func(response map[string]interface{}) ([]map[string]interface{}, error),
	getNextPage func(response *http.Response, body map[string]interface{}) string,
) ([]map[string]interface{}, error) {
	var allItems []map[string]interface{}
	currentPath := initialPath

	for currentPath != "" {
		resp, err := b.DoRaw(ctx, http.MethodGet, currentPath, token, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var responseData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
			// Try parsing as array (some endpoints return arrays directly)
			var items []map[string]interface{}
			if arrErr := json.Unmarshal(bodyBytes, &items); arrErr == nil {
				allItems = append(allItems, items...)
				// For array responses, check Link header for pagination
				if HasNextPage(resp.Header.Get("Link")) {
					currentPath = ParseLinkHeader(resp.Header.Get("Link"))
					continue
				}
				break
			}
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		items, err := extractItems(responseData)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)

		// Get next page URL
		currentPath = getNextPage(resp, responseData)
	}

	return allItems, nil
}

// Error Handling Helpers

// HandleAPIError converts HTTP errors to provider-specific errors.
func HandleAPIError(err error) error {
	httpErr, ok := httpclient.IsHTTPError(err)
	if !ok {
		return err
	}

	switch {
	case httpErr.IsNotFound():
		return ErrRepositoryNotFound
	case httpErr.IsForbidden():
		return ErrPermissionDenied
	case httpErr.IsUnauthorized():
		return ErrAuthenticationFailed
	case httpErr.IsRateLimited():
		return ErrRateLimitExceeded
	default:
		return err
	}
}

// CheckResponseStatus checks the response status and returns appropriate errors.
func CheckResponseStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	httpErr := &httpclient.HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       string(body),
	}

	return HandleAPIError(httpErr)
}

// JSON Helpers

// DecodeJSON decodes a JSON response body into the given target.
func DecodeJSON(resp *http.Response, target interface{}) error {
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	return json.Unmarshal(body, target)
}

// DecodeJSONBytes decodes JSON bytes into the given target.
func DecodeJSONBytes(data []byte, target interface{}) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

// EncodeJSON encodes the given value to JSON bytes.
func EncodeJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// OAuth Token Helpers

// GetOAuthToken retrieves the OAuth access token from the source control data.
// This is used by GitLab and Bitbucket which use OAuth tokens.
func (b *BaseGitProvider) GetOAuthToken() (string, error) {
	if b.sourceControl == nil || b.sourceControl.ProviderData == nil {
		return "", ErrAuthenticationFailed
	}

	token, ok := b.sourceControl.ProviderData["access_token"].(string)
	if !ok || token == "" {
		return "", ErrAuthenticationFailed
	}

	return token, nil
}

// Deployment ID Helpers

// ExtractFloatID extracts an ID from a float64 value in a map.
func ExtractFloatID(data map[string]interface{}, key string) string {
	if idFloat, ok := data[key].(float64); ok {
		return fmt.Sprintf("%.0f", idFloat)
	}
	if idStr, ok := data[key].(string); ok {
		return idStr
	}
	return ""
}

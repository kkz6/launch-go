// Package providers implements DNS provider integrations.
package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/httpclient"
)

// HTTPBaseProvider provides common HTTP client functionality for DNS providers.
// It wraps the httpclient.Client to provide DNS-specific error handling and
// common patterns used across all DNS providers.
type HTTPBaseProvider struct {
	BaseProvider
	client       *httpclient.Client
	providerName string
}

// HTTPBaseConfig contains configuration for creating an HTTPBaseProvider.
type HTTPBaseConfig struct {
	BaseURL      string
	ProviderName string
	Token        string
	AuthScheme   string // defaults to "Bearer"
	Headers      map[string]string
}

// NewHTTPBaseProvider creates a new HTTPBaseProvider with the given configuration.
func NewHTTPBaseProvider(config HTTPBaseConfig) *HTTPBaseProvider {
	authScheme := config.AuthScheme
	if authScheme == "" {
		authScheme = "Bearer"
	}

	opts := []httpclient.Option{
		httpclient.WithAuth(config.Token),
		httpclient.WithAuthScheme(authScheme),
		httpclient.WithHeader("Accept", "application/json"),
	}

	for key, value := range config.Headers {
		opts = append(opts, httpclient.WithHeader(key, value))
	}

	client := httpclient.NewClient(config.BaseURL, opts...)

	p := &HTTPBaseProvider{
		client:       client,
		providerName: config.ProviderName,
	}
	p.SetCredentials(map[string]string{"token": config.Token})

	return p
}

// Client returns the underlying HTTP client.
func (p *HTTPBaseProvider) Client() *httpclient.Client {
	return p.client
}

// SetToken updates the authentication token.
func (p *HTTPBaseProvider) SetToken(token string) {
	p.client.SetAuth(token)
	p.credentials["token"] = token
}

// ProviderName returns the name of the provider.
func (p *HTTPBaseProvider) ProviderName() string {
	return p.providerName
}

// Get performs a GET request and decodes the response into result.
func (p *HTTPBaseProvider) Get(ctx context.Context, path string, result interface{}) error {
	err := p.client.Get(ctx, path, result)
	return p.wrapError(err, "GET", path)
}

// Post performs a POST request and decodes the response into result.
func (p *HTTPBaseProvider) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	err := p.client.Post(ctx, path, body, result)
	return p.wrapError(err, "POST", path)
}

// Put performs a PUT request and decodes the response into result.
func (p *HTTPBaseProvider) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	err := p.client.Put(ctx, path, body, result)
	return p.wrapError(err, "PUT", path)
}

// Delete performs a DELETE request and decodes the response into result.
func (p *HTTPBaseProvider) Delete(ctx context.Context, path string, result interface{}) error {
	err := p.client.Delete(ctx, path, result)
	return p.wrapError(err, "DELETE", path)
}

// DoRequest performs a request using the request builder pattern.
func (p *HTTPBaseProvider) DoRequest(ctx context.Context, req *httpclient.Request, result interface{}) error {
	err := req.Do(ctx, result)
	return p.wrapError(err, "", "")
}

// GetWithQuery performs a GET request with query parameters.
func (p *HTTPBaseProvider) GetWithQuery(ctx context.Context, path string, params map[string]string, result interface{}) error {
	req := p.client.GET(path).WithQueryParams(params)
	err := req.Do(ctx, result)
	return p.wrapError(err, "GET", path)
}

// DoRaw performs a request and returns the raw response for custom handling.
func (p *HTTPBaseProvider) DoRaw(ctx context.Context, method, path string, body interface{}) (*httpclient.Response, error) {
	resp, err := p.client.Request(method, path).WithBody(body).DoResponse(ctx)
	if err != nil {
		return nil, p.wrapError(err, method, path)
	}
	return resp, nil
}

// wrapError wraps HTTP errors in ProviderError for consistent error handling.
func (p *HTTPBaseProvider) wrapError(err error, method, path string) error {
	if err == nil {
		return nil
	}

	httpErr, ok := httpclient.IsHTTPError(err)
	if ok {
		return NewProviderError(p.providerName, httpErr.StatusCode, httpErr.Body, nil)
	}

	message := fmt.Sprintf("request failed: %s %s", method, path)
	if method == "" {
		message = "request failed"
	}
	return NewProviderError(p.providerName, 0, message, err)
}

// ParseErrorResponse extracts error message from a provider-specific error fiberctx.
// This is a helper for providers that have custom error response formats.
func (p *HTTPBaseProvider) ParseErrorResponse(body []byte, extractor func([]byte) (string, error)) error {
	if len(body) == 0 {
		return NewProviderError(p.providerName, 0, "empty error response", nil)
	}

	message, err := extractor(body)
	if err != nil {
		return NewProviderError(p.providerName, 0, string(body), nil)
	}

	return NewProviderError(p.providerName, 0, message, nil)
}

// CheckResponseSuccess checks if a response indicates success based on status code.
func (p *HTTPBaseProvider) CheckResponseSuccess(resp *httpclient.Response, successCodes ...int) error {
	if resp.IsSuccess() {
		return nil
	}

	for _, code := range successCodes {
		if resp.StatusCode == code {
			return nil
		}
	}

	return NewProviderError(p.providerName, resp.StatusCode, resp.String(), nil)
}

// UnmarshalResponse unmarshals a successful response body into the target.
func (p *HTTPBaseProvider) UnmarshalResponse(resp *httpclient.Response, target interface{}) error {
	if !resp.IsSuccess() {
		return NewProviderError(p.providerName, resp.StatusCode, resp.String(), nil)
	}

	if len(resp.Body) == 0 {
		return nil
	}

	if err := json.Unmarshal(resp.Body, target); err != nil {
		return NewProviderError(p.providerName, 0, "failed to parse response", err)
	}

	return nil
}

// BuildRecordData creates a map with common DNS record fields.
// This is a helper for building request bodies when creating/updating records.
func BuildRecordData(record *DNSRecord) map[string]interface{} {
	data := map[string]interface{}{
		"type": record.Type.String(),
		"name": record.Name,
	}

	if record.TTL > 0 {
		data["ttl"] = record.TTL
	}

	if record.Priority != nil {
		data["priority"] = *record.Priority
	}

	if record.Weight != nil {
		data["weight"] = *record.Weight
	}

	if record.Port != nil {
		data["port"] = *record.Port
	}

	if record.Flags != nil {
		data["flags"] = *record.Flags
	}

	if record.Tag != nil {
		data["tag"] = *record.Tag
	}

	if record.Comment != nil && *record.Comment != "" {
		data["comment"] = *record.Comment
	}

	return data
}

// IntPtr creates a pointer to an int value.
func IntPtr(i int) *int {
	return &i
}

// StringPtr creates a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}

// BoolPtr creates a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

// logUpstreamError emits the full upstream HTTP response (status + body) at
// WARN level whenever a provider call fails AND captures the failure to
// Sentry. Centralising it here means every DO/Hetzner/Linode/Vultr/AWS API
// call gets uniform diagnostics in the worker log + Sentry — without it the
// wrapped APIError surfaces "create droplet failed" with no clue which field
// the upstream actually rejected.
//
// Sentry capture is a no-op if Sentry hasn't been initialised (e.g. in tests
// or when SENTRY_DSN isn't set), so we don't need to guard the call.
func logUpstreamError(method, path string, err error) {
	httpErr, ok := httpclient.IsHTTPError(err)
	if !ok {
		return
	}

	log.Warn().
		Str("upstream_method", method).
		Str("upstream_path", path).
		Int("upstream_status", httpErr.StatusCode).
		Str("upstream_status_text", httpErr.Status).
		Str("upstream_body", httpErr.Body).
		Msg("Provider API call failed")

	sentry.WithScope(func(scope *sentry.Scope) {
		// Tags are indexed and filterable in the Sentry UI — pick fields that
		// make sense as facets (method, status code). Path goes in extras
		// because high-cardinality values blow up Sentry's tag indexes.
		scope.SetTag("upstream_method", method)
		scope.SetTag("upstream_status", fmt.Sprintf("%d", httpErr.StatusCode))
		scope.SetLevel(sentry.LevelError)
		scope.SetExtra("upstream_path", path)
		scope.SetExtra("upstream_status_text", httpErr.Status)
		scope.SetExtra("upstream_body", httpErr.Body)
		sentry.CaptureException(err)
	})
}

// BaseCloudProvider provides common HTTP client functionality for cloud providers.
// It wraps the httpclient.Client and provides provider-specific helpers.
type BaseCloudProvider struct {
	keyGenerator sshkey.Generator
	config       config.ProviderConfig
	baseURL      string
}

// NewBaseCloudProvider creates a new BaseCloudProvider.
func NewBaseCloudProvider(keyGenerator sshkey.Generator, providerConfig config.ProviderConfig, baseURL string) BaseCloudProvider {
	return BaseCloudProvider{
		keyGenerator: keyGenerator,
		config:       providerConfig,
		baseURL:      baseURL,
	}
}

// GenerateKeyPair generates a new SSH key pair.
func (p *BaseCloudProvider) GenerateKeyPair() (*KeyPair, error) {
	return p.keyGenerator.Generate()
}

// Plans returns the plans from config.
func (p *BaseCloudProvider) Plans() []config.PlanOption {
	return p.config.Plans
}

// Regions returns the regions from config.
func (p *BaseCloudProvider) Regions() []config.RegionOption {
	return p.config.Regions
}

// NewClient creates a new HTTP client with the given token for API requests.
func (p *BaseCloudProvider) NewClient(token string) *httpclient.Client {
	return httpclient.NewClient(
		p.baseURL,
		httpclient.WithAuth(token),
		httpclient.WithRetry(httpclient.DefaultRetryOptions()),
	)
}

// GetImageFromConfig returns the image ID for an operating system from the provider config.
func (p *BaseCloudProvider) GetImageFromConfig(os types.OperatingSystem, defaultImage string) string {
	osKey := os.String()
	if img, ok := p.config.Images[osKey]; ok {
		if str, ok := img.(string); ok {
			return str
		}
	}
	return defaultImage
}

// ExtractToken extracts and validates a token from credentials.
func ExtractToken(credentials map[string]interface{}) (string, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return "", ErrInvalidCredentials
	}
	return token, nil
}

// GetProviderData safely gets provider data from a map, initializing if nil.
func GetProviderData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}
	return data
}

// GetStringField extracts a string field from a map with a default value.
func GetStringField(data map[string]interface{}, key string, defaultValue string) string {
	if val, ok := data[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

// GetIntField extracts an int field from a map (handles float64 from JSON).
func GetIntField(data map[string]interface{}, key string, defaultValue int) int {
	switch v := data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return defaultValue
	}
}

// GetNestedMap extracts a nested map from a map.
func GetNestedMap(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	val, ok := data[key].(map[string]interface{})
	return val, ok
}

// GetNestedArray extracts a nested array from a map.
func GetNestedArray(data map[string]interface{}, key string) ([]interface{}, bool) {
	val, ok := data[key].([]interface{})
	return val, ok
}

// APIError represents a structured API error fiberctx.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	// Always include the upstream response body when we have one — without it
	// a "create droplet failed" message is opaque and the user can't see which
	// field DO/Hetzner/etc. actually rejected. Cap the body so a runaway HTML
	// error page from a misconfigured proxy doesn't flood the logs.
	const maxBody = 500
	body := e.Body
	if len(body) > maxBody {
		body = body[:maxBody] + "…"
	}

	if e.Message != "" {
		if body != "" {
			return fmt.Sprintf("API error (HTTP %d): %s: %s", e.StatusCode, e.Message, body)
		}
		return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, body)
}

// WrapHTTPError converts an httpclient.HTTPError to a more user-friendly error.
func WrapHTTPError(err error, operation string) error {
	if httpErr, ok := httpclient.IsHTTPError(err); ok {
		switch {
		case httpErr.IsUnauthorized():
			return ErrInvalidCredentials
		case httpErr.IsNotFound():
			return fiberutil.NotFound()
		case httpErr.IsRateLimited():
			return &APIError{
				StatusCode: httpErr.StatusCode,
				Message:    "rate limit exceeded, please try again later",
				Body:       httpErr.Body,
			}
		default:
			return &APIError{
				StatusCode: httpErr.StatusCode,
				Message:    fmt.Sprintf("%s failed", operation),
				Body:       httpErr.Body,
			}
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// CommonCredentialRules returns the standard credential rules for token-based auth.
func CommonCredentialRules() map[string]string {
	return map[string]string{
		"token": "required",
	}
}

// CommonCreateRules returns the standard create rules for plan/region.
func CommonCreateRules() map[string]string {
	return map[string]string{
		"plan":   "required",
		"region": "required",
	}
}

// CommonCredentialData extracts standard credential data (token).
func CommonCredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"token": input["token"],
	}
}

// CommonProviderData extracts standard provider data (plan, region).
func CommonProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"plan":   input["plan"],
		"region": input["region"],
	}
}

// DoGet performs a GET request using the httpclient.
func DoGet(ctx context.Context, client *httpclient.Client, path string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := client.Get(ctx, path, &result); err != nil {
		logUpstreamError(http.MethodGet, path, err)
		return nil, err
	}
	return result, nil
}

// DoPost performs a POST request using the httpclient.
func DoPost(ctx context.Context, client *httpclient.Client, path string, body interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := client.Post(ctx, path, body, &result); err != nil {
		logUpstreamError(http.MethodPost, path, err)
		return nil, err
	}
	return result, nil
}

// DoDelete performs a DELETE request using the httpclient.
// Returns nil error even if the server returns an error (for idempotent deletes).
func DoDelete(ctx context.Context, client *httpclient.Client, path string) error {
	var result map[string]interface{}
	err := client.Delete(ctx, path, &result)
	if err != nil {
		// For deletes, we often want to ignore 404 errors
		if httpErr, ok := httpclient.IsHTTPError(err); ok && httpErr.IsNotFound() {
			return nil
		}
		// Log other failures so we can see what the upstream complained about.
		logUpstreamError(http.MethodDelete, path, err)
	}
	return err
}

// DoDeleteIgnoreErrors performs a DELETE request and ignores all errors.
// Useful for cleanup operations where we don't care if the resource already doesn't exist.
func DoDeleteIgnoreErrors(ctx context.Context, client *httpclient.Client, path string) {
	_ = DoDelete(ctx, client, path)
}

// Response helpers for common provider response patterns

// ExtractServerID extracts a server/instance ID from a response map.
//
// JSON numbers decode into interface{} as float64 by default, and
// fmt.Sprintf("%v", float64(56516756)) produces "5.6516756e+07" — which
// DigitalOcean's API rejects with `invalid key identifiers` when we
// round-trip the SSH-key ID back into a droplet-create request. So we
// type-switch and format integers as decimal strings explicitly.
func ExtractServerID(data map[string]interface{}, key string) string {
	switch v := data[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		// All upstream IDs we've seen are whole numbers; trunc and format.
		return strconv.FormatInt(int64(v), 10)
	case float32:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ValidateConnection tests the connection to a provider's API.
func ValidateConnection(ctx context.Context, client *httpclient.Client, testPath string) error {
	resp, err := client.DoRaw(ctx, http.MethodGet, testPath, nil)
	if err != nil {
		return ErrConnectionFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrInvalidCredentials
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ErrConnectionFailed
	}

	return nil
}

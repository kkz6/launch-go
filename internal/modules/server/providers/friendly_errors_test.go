package providers

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
)

// These tests pin the product-facing copy. They check the *content* of the
// message rather than the exact wording so small editorial tweaks don't
// break the suite, but they verify the message does NOT leak internal
// vocabulary ("worker", "asynq", "panic", raw HTTP bodies).

func TestFriendlyError_NamesProvider(t *testing.T) {
	msg := FriendlyError(types.ProviderDigitalOcean, errors.New("anything"))
	assert.Contains(t, msg, "DigitalOcean",
		"friendly message must name the provider the user picked")
}

func TestFriendlyError_NoLeakedInternals(t *testing.T) {
	// Run a slew of representative errors through the classifier and assert
	// none of them produces a message that exposes internal terms.
	cases := []error{
		errors.New("foo: bar baz"),
		errors.New("context deadline exceeded after 30s"),
		errors.New("dial tcp 10.0.0.1:443: connection refused"),
		ErrInvalidCredentials,
		&APIError{StatusCode: 422, Body: `{"id":"unprocessable_entity","message":"size not available in region"}`},
		&APIError{StatusCode: 500, Body: "<html><body>nginx error page</body></html>"},
	}
	bannedTokens := []string{
		"worker", "asynq", "panic", "stacktrace", "json.Unmarshal",
		"goroutine", "<html>", "{", "}", "[",
	}
	for i, err := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			msg := FriendlyError(types.ProviderDigitalOcean, err)
			low := strings.ToLower(msg)
			for _, token := range bannedTokens {
				assert.NotContains(t, low, strings.ToLower(token),
					"friendly message for %q leaked internal token %q: %s", err, token, msg)
			}
		})
	}
}

func TestFriendlyError_ClassifiesAuthFailure(t *testing.T) {
	msg := FriendlyError(types.ProviderDigitalOcean, ErrInvalidCredentials)
	low := strings.ToLower(msg)
	assert.True(t,
		strings.Contains(low, "credential") || strings.Contains(low, "token"),
		"401-class failures should nudge the user toward credentials, got: %s", msg)
}

func TestFriendlyError_ClassifiesQuotaOrValidation(t *testing.T) {
	apiErr := &APIError{StatusCode: 422, Body: `{"message":"size not available"}`}
	msg := FriendlyError(types.ProviderDigitalOcean, apiErr)
	low := strings.ToLower(msg)
	assert.True(t,
		strings.Contains(low, "region") || strings.Contains(low, "plan") || strings.Contains(low, "size"),
		"422 should suggest changing region/plan, got: %s", msg)
}

func TestFriendlyError_ClassifiesBilling(t *testing.T) {
	apiErr := &APIError{StatusCode: 402, Body: "payment required"}
	msg := FriendlyError(types.ProviderDigitalOcean, apiErr)
	assert.Contains(t, strings.ToLower(msg), "billing",
		"402 should mention billing, got: %s", msg)
}

func TestFriendlyError_ClassifiesRateLimit(t *testing.T) {
	apiErr := &APIError{StatusCode: 429, Body: "rate limited"}
	msg := FriendlyError(types.ProviderDigitalOcean, apiErr)
	low := strings.ToLower(msg)
	assert.True(t,
		strings.Contains(low, "rate") || strings.Contains(low, "wait"),
		"429 should mention rate limiting or waiting, got: %s", msg)
}

func TestFriendlyError_ClassifiesServerError(t *testing.T) {
	apiErr := &APIError{StatusCode: 503, Body: "service unavailable"}
	msg := FriendlyError(types.ProviderDigitalOcean, apiErr)
	low := strings.ToLower(msg)
	assert.True(t,
		strings.Contains(low, "their side") || strings.Contains(low, "temporary"),
		"5xx should blame the upstream and suggest retry, got: %s", msg)
}

func TestFriendlyError_ClassifiesTimeout(t *testing.T) {
	msg := FriendlyError(types.ProviderDigitalOcean, errors.New("context deadline exceeded"))
	low := strings.ToLower(msg)
	assert.True(t,
		strings.Contains(low, "timed out") || strings.Contains(low, "timeout"),
		"timeouts should be classified as timeouts, got: %s", msg)
}

func TestFriendlyError_NilReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", FriendlyError(types.ProviderDigitalOcean, nil))
}

func TestIsHTTPErrorUnwrap_FindsBareHTTPError(t *testing.T) {
	h := &httpclient.HTTPError{StatusCode: 503, Body: "down"}
	got, ok := IsHTTPErrorUnwrap(h)
	assert.True(t, ok)
	assert.Equal(t, 503, got.StatusCode)
}

func TestIsHTTPErrorUnwrap_FindsAPIError(t *testing.T) {
	a := &APIError{StatusCode: 422, Body: `{"msg":"bad"}`}
	got, ok := IsHTTPErrorUnwrap(a)
	assert.True(t, ok)
	assert.Equal(t, 422, got.StatusCode)
}

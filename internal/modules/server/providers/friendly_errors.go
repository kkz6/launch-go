package providers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
)

// FriendlyError converts an upstream provider error into a short, plain-language
// message safe to show in the product UI. The goal is "a non-technical user
// reads this and knows what to do" — no "worker logs", no HTTP method names,
// no opaque error IDs.
//
// The returned message names the cloud provider, explains *what* happened in
// one sentence, and (when we can tell) hints at the user's next step.
// Fall-back is a deliberately generic message so we never leak raw stack
// traces into the UI.
func FriendlyError(provider types.ServerProvider, err error) string {
	if err == nil {
		return ""
	}

	providerLabel := provider.Label()

	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return fmt.Sprintf(
			"%s rejected the API credentials connected to this account. Reconnect the provider with a valid token and try again.",
			providerLabel,
		)
	}

	// Anything wrapping an httpclient.HTTPError carries the upstream response.
	// Bucket it by status code into a friendly message; never surface the raw
	// body — that's what the worker logs / Sentry are for.
	if httpErr, ok := IsHTTPErrorUnwrap(err); ok {
		return friendlyFromHTTP(providerLabel, httpErr)
	}

	// Generic operational failures (timeouts, JSON-decode errors, etc.).
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "context deadline exceeded"),
		strings.Contains(low, "timeout"):
		return fmt.Sprintf(
			"We couldn't reach %s — the request timed out. This is usually temporary; try again in a moment.",
			providerLabel,
		)
	case strings.Contains(low, "dial tcp"),
		strings.Contains(low, "no such host"),
		strings.Contains(low, "connection refused"):
		return fmt.Sprintf(
			"We couldn't connect to %s. This is usually a temporary network issue; please try again.",
			providerLabel,
		)
	case strings.Contains(low, "no public ip available"):
		return fmt.Sprintf(
			"%s is taking longer than expected to assign a public IP. Try again — if it keeps happening, the region may be slow or out of capacity.",
			providerLabel,
		)
	}

	return fmt.Sprintf(
		"Something went wrong while provisioning on %s. We've logged the details for our team — please try again or contact support if it keeps happening.",
		providerLabel,
	)
}

// IsHTTPErrorUnwrap walks the error chain looking for an httpclient.HTTPError,
// since provider helpers usually wrap it in *APIError before returning.
func IsHTTPErrorUnwrap(err error) (*httpclient.HTTPError, bool) {
	for err != nil {
		if h, ok := httpclient.IsHTTPError(err); ok {
			return h, true
		}
		var api *APIError
		if errors.As(err, &api) {
			// APIError doesn't preserve the underlying httpclient.HTTPError,
			// but we still have the status + body to bucket from.
			return &httpclient.HTTPError{
				StatusCode: api.StatusCode,
				Status:     fmt.Sprintf("%d", api.StatusCode),
				Body:       api.Body,
			}, true
		}
		err = errors.Unwrap(err)
	}
	return nil, false
}

func friendlyFromHTTP(providerLabel string, h *httpclient.HTTPError) string {
	switch {
	case h.StatusCode == 401, h.StatusCode == 403:
		return fmt.Sprintf(
			"%s rejected the API credentials connected to this account. Reconnect the provider with a valid token and try again.",
			providerLabel,
		)
	case h.StatusCode == 402:
		return fmt.Sprintf(
			"Your %s account couldn't be charged for this server. Check your billing settings with the provider and try again.",
			providerLabel,
		)
	case h.StatusCode == 422, h.StatusCode == 400:
		// Validation. We'd love to surface DO's "size not available in region"
		// verbatim, but the upstream payloads vary too much to safely parse.
		// Use a friendly message that nudges the user toward common causes.
		return fmt.Sprintf(
			"%s couldn't create this server with the selected region and plan. Try a different region or size — some combinations aren't available on every account.",
			providerLabel,
		)
	case h.StatusCode == 429:
		return fmt.Sprintf(
			"%s is currently rate-limiting requests from this account. Wait a minute and try again.",
			providerLabel,
		)
	case h.StatusCode >= 500:
		return fmt.Sprintf(
			"%s is experiencing an issue on their side (status %d). This is usually temporary — please try again shortly.",
			providerLabel, h.StatusCode,
		)
	}
	return fmt.Sprintf(
		"%s returned an unexpected response (status %d). Please try again, or contact support if it keeps happening.",
		providerLabel, h.StatusCode,
	)
}

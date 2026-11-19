package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitHubUserAccessError carries only safe classification fields. It never
// retains response bodies, credentials, or request URLs.
type GitHubUserAccessError struct {
	StatusCode           int
	Retryable            bool
	AuthorizationInvalid bool
}

func (e *GitHubUserAccessError) Error() string {
	return fmt.Sprintf("GitHub user access failed: status %d", e.StatusCode)
}

func githubUserResponseError(response *http.Response) *GitHubUserAccessError {
	status := response.StatusCode
	failure := &GitHubUserAccessError{
		StatusCode:           status,
		AuthorizationInvalid: status == http.StatusUnauthorized,
		Retryable:            status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError,
	}
	if status == http.StatusForbidden {
		failure.Retryable = response.Header.Get("X-RateLimit-Remaining") == "0" || response.Header.Get("Retry-After") != ""
		var body struct{ Message string }
		if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&body) == nil {
			message := strings.ToLower(body.Message)
			failure.Retryable = failure.Retryable || strings.Contains(message, "rate limit") || strings.Contains(message, "abuse")
		}
	}
	return failure
}

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubRepositoryPermissionDenialsDoNotHideOtherCandidates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		headers map[string]string
		admin   bool
		wantErr bool
	}{
		{name: "administrator", status: http.StatusOK, body: `{"permission":"admin"}`, admin: true},
		{name: "write only", status: http.StatusOK, body: `{"permission":"write"}`},
		{name: "organization policy denies repository", status: http.StatusForbidden, body: `{"message":"Resource not accessible by integration"}`},
		{name: "repository removed", status: http.StatusNotFound, body: `{"message":"Not Found"}`},
		{name: "expired user token", status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`, wantErr: true},
		{name: "primary rate limit", status: http.StatusForbidden, body: `{"message":"API rate limit exceeded"}`, headers: map[string]string{"X-RateLimit-Remaining": "0"}, wantErr: true},
		{name: "secondary rate limit", status: http.StatusForbidden, body: `{"message":"You have exceeded a secondary rate limit"}`, wantErr: true},
		{name: "retry requested", status: http.StatusForbidden, body: `{"message":"Please wait"}`, headers: map[string]string{"Retry-After": "60"}, wantErr: true},
		{name: "too many requests", status: http.StatusTooManyRequests, body: `{}`, wantErr: true},
		{name: "upstream unavailable", status: http.StatusServiceUnavailable, body: `{}`, wantErr: true},
		{name: "malformed permission response", status: http.StatusOK, body: `{`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer user-token", r.Header.Get("Authorization"))
				require.Equal(t, "/repos/AI-AGENT-INDIA-GJC-IDG/repository/collaborators/sponsor/permission", r.URL.Path)
				for name, value := range tc.headers {
					w.Header().Set(name, value)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			provider := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
			admin, err := provider.UserRepositoryAdmin(context.Background(), "user-token", "AI-AGENT-INDIA-GJC-IDG/repository", "sponsor")
			require.Equal(t, tc.admin, admin)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

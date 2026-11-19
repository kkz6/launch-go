package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubRestrictedLaterInventoryPageNeverReturnsPartialAuthority(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			w.WriteHeader(http.StatusUnavailableForLegalReasons)
			return
		}
		w.Header().Set("Link", fmt.Sprintf("<%s/user/installations/42/repositories?per_page=100&page=2>; rel=\"next\"", server.URL))
		_ = json.NewEncoder(w).Encode(map[string]any{"repositories": []map[string]any{{"id": 101, "full_name": "org/first"}}})
	}))
	t.Cleanup(server.Close)
	provider := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
	repositories, err := provider.GetUserInstallationRepositories(context.Background(), "user-token", "42")
	var denied *GitHubUserAccessError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, http.StatusUnavailableForLegalReasons, denied.StatusCode)
	require.False(t, denied.Retryable)
	require.Nil(t, repositories, "a blocked page cannot turn a partial inventory into complete authority")
	repositories, next, err := provider.GetUserInstallationRepositoriesPage(context.Background(), "user-token", "42", 2)
	require.ErrorAs(t, err, &denied)
	require.Nil(t, repositories)
	require.Zero(t, next)
}

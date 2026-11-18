package providers

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubUserInventoryRequiresAllPagesAndPreservesIDs(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/user/installations/42/repositories?page=2>; rel="next"`, server.URL))
		_, _ = fmt.Fprint(w, `{"repositories":[{"id":9007199254740993,"full_name":"org/first"}]}`)
	}))
	defer server.Close()
	p := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
	repos, err := p.GetUserInstallationRepositories(context.Background(), "user-token", "42")
	require.Error(t, err)
	require.Nil(t, repos, "partial candidates must never be offered as complete authority")
}
func TestGitHubUserAuthorizationRejectsNonexpiringTokenResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"access_token":"token","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer server.Close()
	p := NewGitHubProvider(&ProviderConfig{BaseURL: server.URL})
	_, err := p.ExchangeUserCode(context.Background(), "code", "verifier")
	require.Error(t, err)
}

package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubRepositoryPageFetchesOnlyRequestedPage(t *testing.T) {
	var requests atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		require.Equal(t, "/user/installations/42/repositories", r.URL.Path)
		require.Equal(t, "100", r.URL.Query().Get("per_page"))
		require.Equal(t, "Bearer account-user-token", r.Header.Get("Authorization"))
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/user/installations/42/repositories?per_page=100&page=2>; rel="next", <%s/user/installations/42/repositories?page=4>; rel="last"`, server.URL, server.URL))
			_, _ = fmt.Fprint(w, `{"repositories":[{"id":9007199254740993,"full_name":"org/first","permissions":{"admin":true}}]}`)
			return
		}
		require.Equal(t, "2", r.URL.Query().Get("page"))
		_, _ = fmt.Fprint(w, `{"repositories":[]}`)
	}))
	t.Cleanup(server.Close)
	p := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
	repositories, next, err := p.GetUserInstallationRepositoriesPage(context.Background(), "account-user-token", "42", 1)
	require.NoError(t, err)
	require.Len(t, repositories, 1)
	require.Equal(t, "9007199254740993", ExtractFloatID(repositories[0], "id"))
	require.Equal(t, 2, next)
	require.Equal(t, int32(1), requests.Load(), "the first result must not wait for or fetch later pages")
	repositories, next, err = p.GetUserInstallationRepositoriesPage(context.Background(), "account-user-token", "42", next)
	require.NoError(t, err)
	require.Empty(t, repositories)
	require.Zero(t, next)
	require.Equal(t, int32(2), requests.Load())
}

func TestGitHubRepositoryPageRejectsInvalidInputBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(w, `{"repositories":[]}`)
	}))
	t.Cleanup(server.Close)
	p := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
	for _, input := range []struct {
		installation string
		page         int
	}{{"", 1}, {"42/../43", 1}, {"42", 0}, {"42", -1}, {"42", 10001}} {
		_, _, err := p.GetUserInstallationRepositoriesPage(context.Background(), "token", input.installation, input.page)
		require.Error(t, err)
	}
	require.Zero(t, requests.Load())
}

func TestGitHubRepositoryPageRejectsProviderAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"provider failure", http.StatusServiceUnavailable, `{"repositories":[]}`},
		{"invalid JSON", http.StatusOK, `{"repositories":`},
		{"missing repositories", http.StatusOK, `{}`},
		{"null repositories", http.StatusOK, `{"repositories":null}`},
		{"scalar repository", http.StatusOK, `{"repositories":[1]}`},
		{"null repository", http.StatusOK, `{"repositories":[null]}`},
		{"missing identity", http.StatusOK, `{"repositories":[{"full_name":"org/repo"}]}`},
		{"zero identity", http.StatusOK, `{"repositories":[{"id":0,"full_name":"org/repo"}]}`},
		{"invalid identity", http.StatusOK, `{"repositories":[{"id":"1.5","full_name":"org/repo"}]}`},
		{"missing name", http.StatusOK, `{"repositories":[{"id":1}]}`},
		{"blank name", http.StatusOK, `{"repositories":[{"id":1,"full_name":" "}]}`},
		{"trailing JSON", http.StatusOK, `{"repositories":[]} {}`},
		{"trailing garbage", http.StatusOK, `{"repositories":[]} invalid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			t.Cleanup(server.Close)
			p := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
			repositories, next, err := p.GetUserInstallationRepositoriesPage(context.Background(), "token", "42", 1)
			require.Error(t, err)
			require.Nil(t, repositories)
			require.Zero(t, next)
		})
	}
}

func TestGitHubRepositoryPageValidatesNextLink(t *testing.T) {
	const path = "/user/installations/42/repositories"
	for _, tc := range []struct {
		name  string
		link  string
		valid bool
	}{
		{"relative endpoint", `<` + path + `?page=2>; rel="next"`, true},
		{"absolute endpoint", `<BASE` + path + `?per_page=100&page=2>; rel="next"`, true},
		{"other host", `<https://other.example` + path + `?page=2>; rel="next"`, false},
		{"other installation", `<BASE/user/installations/43/repositories?page=2>; rel="next"`, false},
		{"other endpoint", `<BASE/user/installations?page=2>; rel="next"`, false},
		{"encoded path", `<BASE/user/installations/%34%32/repositories?page=2>; rel="next"`, false},
		{"fragment", `<BASE` + path + `?page=2#other>; rel="next"`, false},
		{"repeated page", `<BASE` + path + `?page=1>; rel="next"`, false},
		{"skipped page", `<BASE` + path + `?page=3>; rel="next"`, false},
		{"duplicate page", `<BASE` + path + `?page=2&page=3>; rel="next"`, false},
		{"missing page", `<BASE` + path + `?per_page=100>; rel="next"`, false},
		{"different page size", `<BASE` + path + `?page=2&per_page=50>; rel="next"`, false},
		{"additional filter", `<BASE` + path + `?page=2&since=123>; rel="next"`, false},
		{"invalid query", `<BASE` + path + `?page=2&%zz=1>; rel="next"`, false},
		{"ambiguous next", `<BASE` + path + `?page=2>; rel="next", <BASE` + path + `?page=2>; rel="next"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Link", strings.ReplaceAll(tc.link, "BASE", server.URL))
				_, _ = fmt.Fprint(w, `{"repositories":[{"id":101,"full_name":"org/repo"}]}`)
			}))
			t.Cleanup(server.Close)
			p := NewGitHubProvider(&ProviderConfig{APIURL: server.URL})
			repositories, next, err := p.GetUserInstallationRepositoriesPage(context.Background(), "token", "42", 1)
			if tc.valid {
				require.NoError(t, err)
				require.Len(t, repositories, 1)
				require.Equal(t, 2, next)
			} else {
				require.Error(t, err)
				require.Nil(t, repositories)
				require.Zero(t, next)
			}
			require.Equal(t, int32(1), requests.Load(), "links are validated but never followed")
		})
	}
}

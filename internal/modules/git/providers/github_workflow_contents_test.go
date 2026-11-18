package providers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnsureContentsPreservesWorkflowOwnership(t *testing.T) {
	for _, scenario := range []string{"absent", "identical", "modified", "raced_create"} {
		t.Run(scenario, func(t *testing.T) {
			const workflow = "name: Launch Deploy\n"
			reads, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/access_tokens"):
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "scoped-token", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
					reads++
					require.Equal(t, "release/one", r.URL.Query().Get("ref"))
					if scenario == "absent" || scenario == "raced_create" {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					content := workflow
					if scenario == "modified" {
						content = "name: User edited workflow\n"
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "sha": "existing-sha", "content": base64.StdEncoding.EncodeToString([]byte(content))})
				case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
					writes++
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.NotContains(t, body, "sha", "creation must not adopt a newly created or edited file")
					if scenario == "raced_create" {
						w.WriteHeader(http.StatusConflict)
						return
					}
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]string{"sha": "created-sha"}})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			provider := testProvider(t, server.URL)
			sha, err := provider.EnsureContents(testRepositoryContext(), "100", "kkz6", "test-repo", ".github/workflows/owned.yml", workflow, "Configure workflow", "release/one")
			require.Equal(t, 1, reads)
			switch scenario {
			case "absent":
				require.NoError(t, err)
				require.Equal(t, "created-sha", sha)
				require.Equal(t, 1, writes)
			case "identical":
				require.NoError(t, err)
				require.Equal(t, "existing-sha", sha)
				require.Zero(t, writes)
			case "modified":
				require.ErrorContains(t, err, "workflow was modified")
				require.Zero(t, writes)
			case "raced_create":
				require.Error(t, err)
				require.Equal(t, 1, writes)
			}
		})
	}
}

package services

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/stretchr/testify/require"
)

func TestGitHubDisabledRepositoryCannotUseAdminFastPathOrSave(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "202")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	bound, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: connection.id, Repository: "org/repo202", Operation: access.Clone})
	require.NoError(t, err)
	before := bound.Snapshot()
	review := f.authorize(t, team, user, session, connection.id)
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/installations/42/repositories" {
			archived := candidatePageRepository("101", map[string]any{"admin": true})
			archived["archived"] = true
			disabled := candidatePageRepository("202", map[string]any{"admin": true})
			disabled["disabled"] = true
			_ = json.NewEncoder(w).Encode(map[string]any{"repositories": []map[string]any{archived, disabled, candidatePageRepository("303", map[string]any{"admin": true})}})
			return
		}
		original.ServeHTTP(w, r)
	})
	for _, pages := range [][]int{nil, {1}} {
		candidates, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", pages...)
		require.NoError(t, err)
		eligible := []string{}
		for _, candidate := range candidates.Repositories {
			if !candidate.Unavailable {
				eligible = append(eligible, candidate.GitHubRepositoryID)
			}
		}
		require.Equal(t, []string{"101", "303"}, eligible, "disabled denial overrides admin; archived read access remains allowed")
		if len(pages) > 0 {
			require.NotNil(t, candidates.SelectedRepositories)
			require.Len(t, *candidates.SelectedRepositories, 1)
			require.Equal(t, "202", (*candidates.SelectedRepositories)[0].GitHubRepositoryID)
			require.True(t, (*candidates.SelectedRepositories)[0].Selected)
		} else {
			require.Len(t, candidates.Repositories, 3)
			require.Equal(t, "202", candidates.Repositories[1].GitHubRepositoryID)
			require.True(t, candidates.Repositories[1].Unavailable)
			require.True(t, candidates.Repositories[1].Selected)
		}
		_, err = f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, session, selectionFromReview(t, candidates, "42", "202"))
		require.ErrorIs(t, err, access.ErrAccessDenied, "even a misleading admin permission response cannot select a disabled repo")
		require.NoError(t, access.ValidateSnapshot(ctx, f.db, team, before), "review and rejected Save preserve prior saved intent/version")
	}
}

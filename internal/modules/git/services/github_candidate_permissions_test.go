package services

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/stretchr/testify/require"
)

func TestGitHubCandidatesKeepUsableOrganizationRepositoriesWhenOneIsDenied(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		for _, previouslySelected := range []bool{false, true} {
			t.Run(fmt.Sprintf("status=%d/selected=%t", status, previouslySelected), func(t *testing.T) {
				f := newGitFixture(t)
				var denied atomic.Bool
				original := f.server.Config.Handler
				f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if denied.Load() && r.URL.Path == "/repos/org/repo202/collaborators/sponsor/permission" {
						http.Error(w, `{"message":"Resource not accessible by integration"}`, status)
						return
					}
					original.ServeHTTP(w, r)
				})
				ctx := context.Background()
				team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
				connection := ""
				if previouslySelected {
					connection = f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101", "202").id
				}
				denied.Store(true)
				review := f.authorize(t, team, user, session, connection)
				candidates, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42")
				require.NoError(t, err, "one denied repository must not prevent reviewing the same organization's other repositories")
				byID := map[string]GitHubRepositoryChoice{}
				for _, repository := range candidates.Repositories {
					byID[repository.GitHubRepositoryID] = repository
				}
				require.Contains(t, byID, "101")
				require.Contains(t, byID, "303")
				if previouslySelected {
					require.True(t, byID["202"].Selected)
					require.True(t, byID["202"].Unavailable, "saved selections stay visible for deliberate removal")
				} else {
					require.NotContains(t, byID, "202", "an inaccessible repository cannot become a new selection")
				}
				request := selectionFromReview(t, candidates, "42", "202")
				_, err = f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, session, request)
				require.ErrorIs(t, err, access.ErrAccessDenied)
				request.RepositoryIDs = []string{"101"}
				saved, err := f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, session, request)
				require.NoError(t, err)
				_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: saved.ID, Repository: "org/repo202", Operation: access.Clone})
				require.Error(t, err)
			})
		}
	}
}

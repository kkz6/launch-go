package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func candidatePageRepository(id string, permissions any) map[string]any {
	return map[string]any{"id": json.Number(id), "name": "repo" + id, "full_name": "org/repo" + id, "private": true, "default_branch": "main", "permissions": permissions}
}

func TestGitHubCandidatePagesPreserveSavedSelectionWithoutFetchingLaterPages(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101", "303")
	var saved models.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ? AND full_name = ?", connection.id, "org/repo303").First(&saved).Error)
	require.NoError(t, f.db.Exec("CREATE TABLE sites (id text, address text, team_id text, source_control_id text, source_control_repositories_id bigint, repository_dependency_ids json)").Error)
	require.NoError(t, f.db.Exec("INSERT INTO sites VALUES (?,?,?,?,?,?)", "saved-site", "selected.example", team, connection.id, saved.ID, "[]").Error)
	require.NoError(t, f.db.Exec("INSERT INTO sites VALUES (?,?,?,?,?,?)", "foreign-site", "other.example", "another-team", connection.id, saved.ID, "[]").Error)
	review := f.authorize(t, team, user, session, connection.id)
	var firstCalls, secondCalls, adminCalls atomic.Int32
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/permission") {
			adminCalls.Add(1)
		}
		if r.URL.Path == "/user/installations/42/repositories" && r.URL.Query().Get("page") != "" {
			require.Equal(t, "100", r.URL.Query().Get("per_page"))
			repos := []map[string]any{}
			switch r.URL.Query().Get("page") {
			case "1":
				firstCalls.Add(1)
				repos = append(repos, candidatePageRepository("202", map[string]any{"admin": true}))
				w.Header().Set("Link", fmt.Sprintf("<%s/user/installations/42/repositories?per_page=100&page=2>; rel=\"next\"", f.server.URL))
			case "2":
				secondCalls.Add(1)
				repos = append(repos, candidatePageRepository("101", map[string]any{"admin": true}))
			default:
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
			return
		}
		original.ServeHTTP(w, r)
	})
	first, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", 1)
	require.NoError(t, err)
	require.Equal(t, int32(1), firstCalls.Load())
	require.Zero(t, secondCalls.Load(), "first page must be displayable before fetching page two")
	require.Equal(t, 1, first.Page)
	require.Equal(t, 2, first.NextPage)
	require.Equal(t, &connection.id, first.SourceControlID)
	require.Len(t, first.Repositories, 1)
	require.Equal(t, "202", first.Repositories[0].GitHubRepositoryID)
	require.False(t, first.Repositories[0].Selected)
	require.NotNil(t, first.SelectedRepositories)
	require.Len(t, *first.SelectedRepositories, 2)
	require.Equal(t, "303", (*first.SelectedRepositories)[1].GitHubRepositoryID)
	require.True(t, (*first.SelectedRepositories)[1].Selected)
	require.False(t, (*first.SelectedRepositories)[1].Unavailable, "absence is not established until every page finishes")
	require.Equal(t, []GitHubWorkloadImpact{{Kind: "site", ID: "saved-site", Name: "selected.example"}}, (*first.SelectedRepositories)[1].AffectedWorkloads)
	second, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", 2)
	require.NoError(t, err)
	require.Equal(t, 2, second.Page)
	require.Zero(t, second.NextPage)
	require.Nil(t, second.SelectedRepositories)
	require.Len(t, second.Repositories, 1)
	require.Equal(t, "101", second.Repositories[0].GitHubRepositoryID)
	require.True(t, second.Repositories[0].Selected)
	require.Equal(t, first.ConnectionGeneration, second.ConnectionGeneration)
	require.Equal(t, first.SelectionVersion, second.SelectionVersion)
	require.Zero(t, adminCalls.Load(), "typed per-repository user permissions avoid per-repo requests")
	require.NoError(t, f.db.First(&saved, saved.ID).Error)
	require.True(t, saved.Selected, "browsing pages cannot remove an unseen saved selection")
}

func TestGitHubCandidatePagePermissionsAreTypedAndSaveRechecksAuthority(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	review := f.authorize(t, team, user, session, "")
	var adminCalls atomic.Int32
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/installations/42/repositories" && r.URL.Query().Get("page") != "" {
			repos := []map[string]any{
				candidatePageRepository("101", map[string]any{"admin": true}),
				candidatePageRepository("202", map[string]any{"admin": false}),
				candidatePageRepository("303", map[string]any{"admin": "true"}),
				candidatePageRepository("404", nil),
				candidatePageRepository("505", "admin"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/permission") {
			adminCalls.Add(1)
			if strings.Contains(r.URL.Path, "/repo303/") || strings.Contains(r.URL.Path, "/repo505/") {
				_ = json.NewEncoder(w).Encode(map[string]any{"permission": "read"})
				return
			}
		}
		original.ServeHTTP(w, r)
	})
	page, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", 1)
	require.NoError(t, err)
	require.Len(t, page.Repositories, 2)
	require.Equal(t, "101", page.Repositories[0].GitHubRepositoryID)
	require.Equal(t, "404", page.Repositories[1].GitHubRepositoryID)
	require.Equal(t, int32(3), adminCalls.Load(), "only missing or malformed booleans need fallback verification")
	require.NotNil(t, page.SelectedRepositories)
	require.Empty(t, *page.SelectedRepositories)
	f.admin = false
	_, err = f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, session, GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}})
	require.ErrorIs(t, err, access.ErrAccessDenied, "display permission never bypasses a fresh Save authorization check")
	var grants int64
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&grants).Error)
	require.Zero(t, grants)
}

func TestGitHubCandidatePagesRetainAttemptAndConnectionIsolation(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	f.otherInstallation = true
	connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	review := f.authorize(t, team, user, session, connection.id)
	for _, args := range [][4]string{{team, user, "switched-session", "42"}, {"team-b00000000000000000000", user, session, "42"}, {team, "user-b00000000000000000000", session, "42"}, {team, user, session, "43"}} {
		_, err := f.service.GitHubCandidates(ctx, review.AttemptID, args[0], args[1], args[2], args[3], 2)
		require.Error(t, err)
	}
	_, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", 1)
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&models.SourceControl{}).Where("id = ?", connection.id).Update("selection_version", gorm.Expr("selection_version + 1")).Error)
	_, err = f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", 2)
	require.ErrorIs(t, err, access.ErrStaleAccess)
	for _, page := range []int{0, -1, 10001} {
		_, err = f.service.GitHubCandidates(ctx, review.AttemptID, team, user, session, "42", page)
		require.Error(t, err)
	}
}

func TestGitHubCandidatePageFailureCanRetryWithoutChangingSelection(t *testing.T) {
	f := newGitFixture(t)
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	review := f.authorize(t, team, user, session, "")
	var failed atomic.Bool
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/installations/42/repositories" && r.URL.Query().Get("page") == "2" && !failed.Swap(true) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		original.ServeHTTP(w, r)
	})
	_, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 2)
	require.Error(t, err)
	page, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 2)
	require.NoError(t, err)
	require.Len(t, page.Repositories, 3)
	require.Nil(t, page.SelectedRepositories)
	var grants int64
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&grants).Error)
	require.Zero(t, grants)
}

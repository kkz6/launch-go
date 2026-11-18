package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/stretchr/testify/require"
)

// Build another provider against the same accounts so a test can change the
// upstream response without changing the shared fixture server's live handler.
func sponsorReviewProvider(t *testing.T, f *gitFixture, handler http.HandlerFunc) (*access.Resolver, *providers.GitHubProvider) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	config := *f.provider.Config()
	config.APIURL, config.BaseURL = server.URL, server.URL
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &config)
	provider, err := factory.GetProvider(providers.GitProviderGitHub)
	require.NoError(t, err)
	return access.NewResolver(f.db, factory), provider.(*providers.GitHubProvider)
}

func TestGitHubStaleSponsorReverificationPreservesSelectionAndOtherAccount(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "202")
	var otherRepository models.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ?", b.id).First(&otherRepository).Error)
	var otherAuthorization models.GitHubAuthorization
	require.NoError(t, f.db.Where("user_id = ?", userB).First(&otherAuthorization).Error)
	past := time.Now().Add(-6 * time.Minute)
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", a.id).Update("verified_at", past).Error)

	require.NoError(t, f.db.Model(&models.GitHubAuthorization{}).Where("user_id = ?", userA).Update("expires_at", time.Now().Add(-time.Minute)).Error)
	var refreshChecks, inventoryChecks, permissionChecks, installationChecks atomic.Int32
	resolver, provider := sponsorReviewProvider(t, f, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			refreshChecks.Add(1)
		case "/user/installations/42/repositories":
			inventoryChecks.Add(1)
		case "/repos/org/repo101/collaborators/sponsor/permission":
			permissionChecks.Add(1)
		case "/app/installations/42":
			installationChecks.Add(1)
		}
		f.serve(w, r)
	})
	grant, err := resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err)
	require.EqualValues(t, 1, refreshChecks.Load())
	require.EqualValues(t, 1, inventoryChecks.Load())
	require.EqualValues(t, 1, permissionChecks.Load())
	require.EqualValues(t, 1, installationChecks.Load())
	token, err := provider.GetInstallationToken(grant.Context(ctx), "42")
	require.NoError(t, err)
	require.Equal(t, "installation-token", token)
	require.Len(t, f.minted, 1)
	require.Equal(t, []any{float64(101)}, f.minted[0]["repository_ids"])
	require.Equal(t, map[string]any{"contents": "read"}, f.minted[0]["permissions"])
	var selected []models.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ? AND selected = ?", a.id, true).Find(&selected).Error)
	require.Len(t, selected, 1, "reverification must not import other installation repositories")
	require.Equal(t, "101", selected[0].GitHubRepositoryID)
	require.True(t, selected[0].Available)
	require.NotNil(t, selected[0].VerifiedAt)
	require.True(t, selected[0].VerifiedAt.After(past))
	var unchangedRepository models.SourceControlRepository
	require.NoError(t, f.db.First(&unchangedRepository, "id = ?", otherRepository.ID).Error)
	require.Equal(t, otherRepository, unchangedRepository)
	var unchangedAuthorization models.GitHubAuthorization
	require.NoError(t, f.db.First(&unchangedAuthorization, "id = ?", otherAuthorization.ID).Error)
	require.Equal(t, otherAuthorization, unchangedAuthorization)
}

func TestGitHubStaleSponsorRejectsUpstreamSuspensionBeforeCredentialIssuance(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
	past := time.Now().Add(-6 * time.Minute)
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", a.id).Update("verified_at", past).Error)
	resolver, _ := sponsorReviewProvider(t, f, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/installations/42" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 42, "app_id": 7, "suspended_at": time.Now().UTC().Format(time.RFC3339),
				"account": map[string]any{"id": 10, "login": "org", "type": "Organization"},
			})
			return
		}
		f.serve(w, r)
	})
	grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: a.id, Repository: "org/repo101", Operation: access.Clone})
	require.ErrorIs(t, err, access.ErrAccessDenied)
	require.Nil(t, grant)
	require.Empty(t, f.minted)
	var leaseCount int64
	require.NoError(t, f.db.Model(&models.GitHubCredentialLease{}).Count(&leaseCount).Error)
	require.Zero(t, leaseCount)
	var repository models.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ?", a.id).First(&repository).Error)
	require.True(t, repository.Selected, "suspension retains saved consent for later review")
	require.WithinDuration(t, past, *repository.VerifiedAt, time.Millisecond, "suspension must not extend verified access")
}

func TestGitHubFailedUserTokenRefreshCannotIssueRepositoryCredentials(t *testing.T) {
	for _, tc := range []struct {
		name          string
		refreshExpiry time.Duration
		status        int
		response      map[string]any
		refreshCalls  int32
	}{
		{name: "refresh token expired", refreshExpiry: -time.Minute, status: http.StatusOK},
		{name: "refresh authorization rejected", refreshExpiry: time.Hour, status: http.StatusOK, response: map[string]any{"error": "bad_refresh_token"}, refreshCalls: 1},
		{name: "temporary provider failure", refreshExpiry: time.Hour, status: http.StatusServiceUnavailable, refreshCalls: 1},
		{name: "incomplete token rotation", refreshExpiry: time.Hour, status: http.StatusOK, response: map[string]any{"access_token": "unusable-new-token", "expires_in": 3600}, refreshCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
			teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
			a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
			b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "202")
			past := time.Now().Add(-6 * time.Minute)
			require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", a.id).Update("verified_at", past).Error)
			require.NoError(t, f.db.Model(&models.GitHubAuthorization{}).Where("user_id = ?", userA).Updates(map[string]any{
				"expires_at": time.Now().Add(-time.Minute), "refresh_expires_at": time.Now().Add(tc.refreshExpiry),
			}).Error)
			var before models.GitHubAuthorization
			require.NoError(t, f.db.Where("user_id = ?", userA).First(&before).Error)
			var refreshCalls atomic.Int32
			resolver, provider := sponsorReviewProvider(t, f, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/login/oauth/access_token" {
					refreshCalls.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_ = json.NewEncoder(w).Encode(tc.response)
					return
				}
				f.serve(w, r)
			})
			grant, err := resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo101", Operation: access.Clone})
			require.Error(t, err)
			require.Nil(t, grant)
			require.Equal(t, tc.refreshCalls, refreshCalls.Load())
			require.Empty(t, f.minted, "failed user authorization must never fall back to an App token")
			var leases int64
			require.NoError(t, f.db.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ?", a.id).Count(&leases).Error)
			require.Zero(t, leases)
			var after models.GitHubAuthorization
			require.NoError(t, f.db.First(&after, "id = ?", before.ID).Error)
			require.Equal(t, before, after, "failed rotation must not partially overwrite the saved authorization")
			var repository models.SourceControlRepository
			require.NoError(t, f.db.Where("source_control_id = ?", a.id).First(&repository).Error)
			require.WithinDuration(t, past, *repository.VerifiedAt, time.Millisecond)
			otherGrant, err := resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo202", Operation: access.Clone})
			require.NoError(t, err, "another account's authorization must remain usable")
			token, err := provider.GetInstallationToken(otherGrant.Context(ctx), "42")
			require.NoError(t, err)
			require.NotEmpty(t, token)
			require.Len(t, f.minted, 1)
			require.Equal(t, []any{float64(202)}, f.minted[0]["repository_ids"])
		})
	}
}

package services

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
)

func restrictRuntimeRepositoryPermission(f *gitFixture, fullName string) (*atomic.Bool, *atomic.Int32) {
	enabled, denials := &atomic.Bool{}, &atomic.Int32{}
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enabled.Load() && r.URL.Path == "/repos/"+fullName+"/collaborators/sponsor/permission" {
			denials.Add(1)
			http.Error(w, `{"message":"Repository access blocked"}`, http.StatusUnavailableForLegalReasons)
			return
		}
		original.ServeHTTP(w, r)
	})
	return enabled, denials
}

func TestGitHubRuntime451InvalidatesOnlyRestrictedRepository(t *testing.T) {
	f := newGitFixture(t)
	restricted, denials := restrictRuntimeRepositoryPermission(f, "org/repo101")
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101", "202")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "303")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	requests := []access.Request{
		{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo101", Operation: access.Clone},
		{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo202", Operation: access.Clone},
		{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo303", Operation: access.Clone},
	}
	grants := make([]*access.RepositoryAccess, len(requests))
	for i, request := range requests {
		grant, err := resolver.Resolve(ctx, request)
		require.NoError(t, err)
		_, err = f.provider.GetInstallationToken(grant.Context(ctx), "42")
		require.NoError(t, err)
		grants[i] = grant
	}
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("id = ?", grants[0].Snapshot().RepositoryID).Update("verified_at", time.Now().Add(-6*time.Minute)).Error)
	restricted.Store(true)
	_, err := resolver.Resolve(ctx, requests[0])
	require.ErrorIs(t, err, access.ErrAccessDenied)
	require.Equal(t, int32(1), denials.Load())
	require.Len(t, f.minted, 3, "a denied verification cannot issue another installation credential")
	require.Equal(t, 1, f.revoked, "only the restricted repository's issued credential is revoked")
	for i, grant := range grants {
		var repository models.SourceControlRepository
		var lease models.GitHubCredentialLease
		require.NoError(t, f.db.First(&repository, "id = ?", grant.Snapshot().RepositoryID).Error)
		require.NoError(t, f.db.Where("source_control_id = ? AND repository_id = ?", grant.Snapshot().ConnectionID, grant.Snapshot().RepositoryID).First(&lease).Error)
		require.True(t, repository.Selected, "restriction preserves the user's saved selection")
		if i == 0 {
			require.False(t, repository.Available)
			require.Equal(t, grant.Snapshot().GrantVersion+1, repository.GrantVersion)
			require.Equal(t, "revoked", lease.Status)
			require.Empty(t, lease.Token)
			require.Error(t, grant.Validate(ctx))
		} else {
			require.True(t, repository.Available)
			require.Equal(t, grant.Snapshot().GrantVersion, repository.GrantVersion)
			require.Equal(t, "released", lease.Status)
			require.NotEmpty(t, lease.Token)
			require.NoError(t, grant.Validate(ctx), "other repository and account snapshots remain valid")
		}
	}
	for _, team := range []string{teamA, teamB} {
		var connection models.SourceControl
		require.NoError(t, f.db.Where("team_id = ?", team).First(&connection).Error)
		require.Equal(t, "active", connection.Status)
		require.Equal(t, uint64(1), connection.Generation)
	}
}

func TestGitHubCleanup451CannotMintCredentialOrReleaseOwnership(t *testing.T) {
	f := newGitFixture(t)
	restricted, denials := restrictRuntimeRepositoryPermission(f, "org/repo101")
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "101", "202")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	owner, err := resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	competitor, err := resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	unaffected, err := resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo202", Operation: access.Clone})
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(unaffected.Context(ctx), "42")
	require.NoError(t, err)
	resources := []access.Resource{{Kind: "secret", Name: "LAUNCH_TEST_DEPLOY_TOKEN"}}
	id, err := access.RegisterCleanup(ctx, f.db, a.id, owner.Snapshot().RepositoryID, owner.Snapshot(), resources)
	require.NoError(t, err)
	require.NoError(t, access.ActivateCleanupManifest(ctx, f.db, id))
	cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
	require.NoError(t, err)
	restricted.Store(true)
	token, err := f.provider.GetInstallationToken(cleanup.Context(ctx), "42")
	require.ErrorIs(t, err, access.ErrAccessDenied)
	require.Empty(t, token)
	require.Equal(t, int32(1), denials.Load())
	require.Len(t, f.minted, 1, "cleanup must fail before minting an installation token")
	var count int64
	require.NoError(t, f.db.Model(&models.GitHubCredentialLease{}).Where("cleanup_manifest_id = ?", id).Count(&count).Error)
	require.Zero(t, count, "denied cleanup cannot create a credential lease")
	require.NoError(t, cleanup.Fail(ctx))
	var manifest models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
	require.Equal(t, "failed_cleanup", manifest.Status, "unremoved resources must retain their ownership reservation")
	_, err = access.RegisterCleanup(ctx, f.db, b.id, competitor.Snapshot().RepositoryID, competitor.Snapshot(), resources)
	require.ErrorIs(t, err, access.ErrAccessDenied)
	require.NoError(t, unaffected.Validate(ctx))
	var lease models.GitHubCredentialLease
	require.NoError(t, f.db.Where("source_control_id = ? AND repository_id = ?", b.id, unaffected.Snapshot().RepositoryID).First(&lease).Error)
	require.Equal(t, "released", lease.Status)
	require.NotEmpty(t, lease.Token)
	require.Zero(t, f.revoked, "failed cleanup must not revoke another account's credential")
}

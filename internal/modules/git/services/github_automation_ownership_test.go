package services

import (
	"context"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/stretchr/testify/require"
)

func TestGitHubAutomationOwnershipPreservesOtherAccountSelection(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	aGrant, err := resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: a.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	bGrant, err := resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(aGrant.Context(ctx), "42")
	require.Error(t, err, "write credentials require ownership before remote mutation")
	require.Empty(t, f.minted)
	resources := []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}}
	aManifest, err := access.RegisterCleanup(ctx, f.db, a.id, aGrant.Snapshot().RepositoryID, aGrant.Snapshot(), resources)
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(aGrant.Context(ctx), "42")
	require.NoError(t, err)
	_, err = access.RegisterCleanup(ctx, f.db, b.id, bGrant.Snapshot().RepositoryID, bGrant.Snapshot(), resources)
	require.Error(t, err, "different accounts cannot own repository-wide CI resources")
	_, err = f.provider.GetInstallationToken(bGrant.Context(ctx), "42")
	require.Error(t, err, "a capability resolved before another account's claim cannot bypass it")
	_, err = resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo101", Operation: access.ActionsRead})
	require.Error(t, err, "another account's CI ownership also protects its logs")
	_, err = resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err, "CI ownership does not change B's explicit clone grant")
	// Unrelated B selection changes must leave A's CI credentials usable.
	f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", b.id), userB, "session-b", "101", "202")
	_, err = f.provider.GetInstallationToken(aGrant.Context(ctx), "42")
	require.NoError(t, err)
	// Same account may have more than one workload on its repository.
	_, err = access.RegisterCleanup(ctx, f.db, a.id, aGrant.Snapshot().RepositoryID, aGrant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_TWO"}})
	require.NoError(t, err)
	require.NoError(t, f.service.Disconnect(ctx, a.id, teamA, userA))
	_, err = access.RegisterCleanup(ctx, f.db, b.id, bGrant.Snapshot().RepositoryID, bGrant.Snapshot(), resources)
	require.Error(t, err, "pending cleanup must retain ownership until remote resources are removed")
	var manifests []models.GitHubCleanupManifest
	require.NoError(t, f.db.Where("source_control_id = ?", a.id).Find(&manifests).Error)
	for _, manifest := range manifests {
		cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), manifest.ID)
		require.NoError(t, err)
		require.NoError(t, cleanup.Complete(ctx))
	}
	var completed models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&completed, "id = ?", aManifest).Error)
	require.Equal(t, "completed", completed.Status)
	_, err = access.RegisterCleanup(ctx, f.db, b.id, bGrant.Snapshot().RepositoryID, bGrant.Snapshot(), resources)
	require.NoError(t, err, "B may explicitly enable CI after A's cleanup finishes")
}

func TestGitHubPostgresConcurrentAutomationClaimsHaveOneOwner(t *testing.T) {
	f := newGitFixtureDatabase(t, true)
	ctx := context.Background()
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	grants := []*access.RepositoryAccess{}
	for _, suffix := range []string{"a", "b"} {
		team, user, session := "team-"+suffix+"00000000000000000000", "user-"+suffix+"00000000000000000000", "session-"+suffix
		connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
		grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: connection.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
		require.NoError(t, err)
		grants = append(grants, grant)
	}
	ready := make(chan struct{})
	results := make(chan error, len(grants))
	for _, grant := range grants {
		go func(grant *access.RepositoryAccess) {
			<-ready
			_, err := access.RegisterCleanup(ctx, f.db, grant.Snapshot().ConnectionID, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}})
			results <- err
		}(grant)
	}
	close(ready)
	successes := 0
	for range grants {
		if <-results == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "only one launchctl account can claim repository-wide automation")
	var count int64
	require.NoError(t, f.db.Model(&models.GitHubCleanupManifest{}).Where("status = ?", "registered").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestGitHubIncompleteCleanupAlwaysRetainsAutomationOwnership(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-b00000000000000000000", "user-b00000000000000000000"
	b := f.selectRepos(t, f.authorize(t, team, user, "session-b", ""), user, "session-b", "101")
	grant, err := access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: b.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	expired := time.Now().Add(-time.Hour)
	foreign := models.GitHubCleanupManifest{TeamID: "other-team", GitHubRepositoryID: "101", InstallationID: "another-installation", Status: "pending", ExpiresAt: &expired}
	require.NoError(t, f.db.Create(&foreign).Error)
	for _, status := range []string{"pending", "failed_cleanup", "failed_manual", "unknown"} {
		require.NoError(t, f.db.Model(&foreign).Update("status", status).Error)
		_, err := access.RegisterCleanup(ctx, f.db, b.id, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}})
		require.Error(t, err, "only completed cleanup releases ownership, including across installations: %s", status)
	}
}

func TestGitHubCleanupRegistrationUsesCompleteAuthoritySnapshot(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	sc := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	request := access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.ActionsBootstrap}
	grant, err := resolver.Resolve(ctx, request)
	require.NoError(t, err)
	resources := []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}}
	old, err := access.RegisterCleanup(ctx, f.db, sc.id, grant.Snapshot().RepositoryID, grant.Snapshot(), resources)
	require.NoError(t, err)
	// A recovered upstream authority has a new generation even when this
	// workload's source connection, grant and remote resource names are stable.
	require.NoError(t, f.db.Model(&models.GitHubAuthorization{}).Where("id = ?", grant.Snapshot().AuthorizationID).Update("generation", grant.Snapshot().AuthorizationGeneration+1).Error)
	current, err := resolver.Resolve(ctx, request)
	require.NoError(t, err)
	registered, err := access.RegisterCleanup(ctx, f.db, sc.id, current.Snapshot().RepositoryID, current.Snapshot(), resources)
	require.NoError(t, err)
	require.NotEqual(t, old, registered, "stale authority must not be reused for the new claim")
	_, err = f.provider.GetInstallationToken(current.Context(ctx), "42")
	require.NoError(t, err)
}

func TestGitHubCleanupCompletionIgnoresOtherAccountsRevocationFailure(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	sc := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	id, err := access.RegisterCleanup(ctx, f.db, sc.id, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}})
	require.NoError(t, err)
	require.NoError(t, f.service.Disconnect(ctx, sc.id, team, user))
	cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(cleanup.Context(ctx), "42")
	require.NoError(t, err)
	f.revokeFailToken = "another-accounts-failed-token"
	foreign := models.GitHubCredentialLease{TeamID: "other-team", SourceControlID: "other-connection", Token: dbtype.EncryptedString(f.revokeFailToken), ExpiresAt: time.Now().Add(time.Hour), Status: "revoke_pending"}
	require.NoError(t, f.db.Create(&foreign).Error)
	require.NoError(t, cleanup.Complete(ctx))
	var manifest models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
	require.Equal(t, "completed", manifest.Status)
	require.NoError(t, f.db.First(&foreign, "id = ?", foreign.ID).Error)
	require.Equal(t, "revoke_pending", foreign.Status)
	require.Equal(t, 1, f.revoked, "only the cleanup's own credential is revoked synchronously")
	require.Error(t, resolver.RevokePending(ctx), "the scheduled global retry still handles the other account independently")
}

package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/stretchr/testify/require"
)

func TestGitHubCleanupRevokedDuringIssuanceNeverReturnsCredential(t *testing.T) {
	for _, disconnect := range []bool{true, false} {
		name := "cleanup attempt failed"
		if disconnect {
			name = "connection disconnected"
		}
		t.Run(name, func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			team, user := "team-a00000000000000000000", "user-a00000000000000000000"
			connection := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
			resolver := access.NewResolver(f.db, f.service.ProviderFactory())
			grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: connection.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
			require.NoError(t, err)
			manifestID, err := access.RegisterCleanup(ctx, f.db, connection.id, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}})
			require.NoError(t, err)
			require.NoError(t, access.ActivateCleanupManifest(ctx, f.db, manifestID))
			cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), manifestID)
			require.NoError(t, err)

			foreign := models.GitHubCredentialLease{TeamID: "team-b00000000000000000000", SourceControlID: "other-connection", Token: dbtype.EncryptedString("other-account-token"), ExpiresAt: time.Now().Add(time.Hour), Status: "released"}
			require.NoError(t, f.db.Create(&foreign).Error)
			require.NoError(t, f.db.First(&foreign, "id = ?", foreign.ID).Error)
			// Fail the immediate provider revocation to prove that the late token
			// remains tracked for retry, rather than escaping issuance accounting.
			f.revokeFailToken = "installation-token"
			revocationResult := make(chan error, 1)
			f.beforeToken = func() {
				if disconnect {
					revocationResult <- resolver.Disconnect(ctx, connection.id, team)
					return
				}
				// A failed attempt remains retryable, but a credential being issued
				// for the closed attempt must not be released to its caller.
				revocationResult <- cleanup.Fail(ctx)
			}
			token, err := f.provider.GetInstallationToken(cleanup.Context(ctx), "42")
			require.NoError(t, <-revocationResult)
			require.ErrorIs(t, err, access.ErrStaleAccess)
			require.Empty(t, token)
			require.Equal(t, 1, f.revoked)
			require.Len(t, f.minted, 1)
			require.Equal(t, []any{float64(101)}, f.minted[0]["repository_ids"])
			require.Equal(t, map[string]any{"secrets": "write"}, f.minted[0]["permissions"])

			var lease models.GitHubCredentialLease
			require.NoError(t, f.db.Where("cleanup_manifest_id = ?", manifestID).First(&lease).Error)
			require.Equal(t, "revoke_pending", lease.Status)
			require.Equal(t, "installation-token", lease.Token.String())
			require.True(t, lease.ExpiresAt.After(time.Now()))
			f.revokeFailToken = ""
			require.NoError(t, resolver.RevokePending(ctx))
			require.NoError(t, f.db.First(&lease, "id = ?", lease.ID).Error)
			require.Equal(t, "revoked", lease.Status)
			require.Empty(t, lease.Token)
			require.Equal(t, 2, f.revoked)
			var unchanged models.GitHubCredentialLease
			require.NoError(t, f.db.First(&unchanged, "id = ?", foreign.ID).Error)
			require.Equal(t, foreign, unchanged, "cleanup cannot revoke another account's credential")
		})
	}
}

func TestGitHubSingleWorkloadCleanupPreservesOtherWorkloadsAndAccounts(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "101", "202")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	register := func(team, connection, repository, secret string) (*access.RepositoryAccess, string) {
		grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: connection, Repository: repository, Operation: access.ActionsBootstrap})
		require.NoError(t, err)
		id, err := access.RegisterCleanup(ctx, f.db, connection, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: secret}})
		require.NoError(t, err)
		return grant, id
	}
	first, firstID := register(teamA, a.id, "org/repo101", "LAUNCH_TOKEN_ONE")
	sibling, siblingID := register(teamA, a.id, "org/repo101", "LAUNCH_TOKEN_TWO")
	other, otherID := register(teamB, b.id, "org/repo202", "LAUNCH_TOKEN_OTHER")
	for _, grant := range []*access.RepositoryAccess{sibling, other} {
		_, err := f.provider.GetInstallationToken(grant.Context(ctx), "42")
		require.NoError(t, err)
	}
	var existingLeases []models.GitHubCredentialLease
	require.NoError(t, f.db.Order("id").Find(&existingLeases).Error)
	require.Len(t, existingLeases, 2)

	require.NoError(t, access.ActivateCleanupManifest(ctx, f.db, firstID))
	var activated models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&activated, "id = ?", firstID).Error)
	require.Equal(t, "pending", activated.Status)
	require.NotNil(t, activated.ExpiresAt)
	require.True(t, activated.ExpiresAt.After(time.Now()))
	require.NoError(t, access.ActivateCleanupManifest(ctx, f.db, firstID))
	var repeated models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&repeated, "id = ?", firstID).Error)
	require.Equal(t, activated, repeated, "retries must not extend cleanup authority")
	cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), firstID)
	require.NoError(t, err)
	require.NoError(t, cleanup.ValidateRequest(ctx, http.MethodDelete, "/repos/org/repo101/actions/secrets/LAUNCH_TOKEN_ONE", nil))
	for _, path := range []string{"/repos/org/repo101/actions/secrets/LAUNCH_TOKEN_TWO", "/repos/org/repo202/actions/secrets/LAUNCH_TOKEN_OTHER"} {
		require.ErrorIs(t, cleanup.ValidateRequest(ctx, http.MethodDelete, path, nil), access.ErrAccessDenied)
	}
	_, err = f.provider.GetInstallationToken(cleanup.Context(ctx), "42")
	require.NoError(t, err)
	require.NoError(t, cleanup.Complete(ctx))
	require.NoError(t, access.ActivateCleanupManifest(ctx, f.db, firstID))
	var completed models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&completed, "id = ?", firstID).Error)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 1, f.revoked, "completion revokes only this workload's cleanup credential")
	for _, id := range []string{siblingID, otherID} {
		var manifest models.GitHubCleanupManifest
		require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
		require.Equal(t, "registered", manifest.Status)
		require.Nil(t, manifest.ExpiresAt)
		_, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
		require.Error(t, err, "another workload has not opted into cleanup")
	}
	for _, before := range existingLeases {
		var after models.GitHubCredentialLease
		require.NoError(t, f.db.First(&after, "id = ?", before.ID).Error)
		require.Equal(t, before, after)
	}
	for _, grant := range []*access.RepositoryAccess{first, sibling, other} {
		team := teamA
		if grant == other {
			team = teamB
		}
		require.NoError(t, access.ValidateSnapshot(ctx, f.db, team, grant.Snapshot()))
	}
	_, err = resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err, "another account's selection of the same repository remains usable")
	for _, grant := range []*access.RepositoryAccess{sibling, other} {
		_, err := f.provider.GetInstallationToken(grant.Context(ctx), "42")
		require.NoError(t, err, "unrelated automation continues after one workload is disabled")
	}
}

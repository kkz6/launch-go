package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/database/serializers"
	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sponsorDeletionFixture struct {
	db          *gorm.DB
	a, b        *gitaccess.RepositoryAccess
	manifestIDs []string
	leases      []gitmodels.GitHubCredentialLease
}

func newSponsorDeletionFixture(t *testing.T) sponsorDeletionFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	f := sponsorDeletionFixture{db: db}
	f.a = githubfixture.SelectedRepository(t, db, "team-a", "shared/frontend", map[string]any{})
	f.b = githubfixture.SelectedRepository(t, db, "team-b", "shared/backend", map[string]any{})
	require.NoError(t, db.AutoMigrate(&gitmodels.GitHubCleanupManifest{}))
	// Two local logins share the upstream identity and installation, but each
	// has its own authorization, selected repository, and issued credentials.
	require.NoError(t, db.Model(f.b.SourceControl).Updates(map[string]any{
		"installation_id": f.a.SourceControl.InstallationID, "git_hub_installation_id": f.a.SourceControl.GitHubInstallationID,
	}).Error)
	for _, grant := range []*gitaccess.RepositoryAccess{f.a, f.b} {
		require.NoError(t, db.Model(&gitmodels.GitHubAuthorization{}).Where("id = ?", grant.Snapshot().AuthorizationID).Updates(map[string]any{
			"git_hub_user_id": "same-github-user", "access_token": dbtype.EncryptedString("access-" + grant.SourceControl.UserID), "refresh_token": dbtype.EncryptedString("refresh-" + grant.SourceControl.UserID),
		}).Error)
		manifestID, err := gitaccess.RegisterCleanup(context.Background(), db, grant.SourceControl.ID, grant.Snapshot().RepositoryID, grant.Snapshot(), []gitaccess.Resource{{Kind: "secret", Name: "LAUNCH_SELECTED_REPOSITORY"}})
		require.NoError(t, err)
		f.manifestIDs = append(f.manifestIDs, manifestID)
		for _, status := range []string{"issuing", "released", "revoked", "expired"} {
			lease := gitmodels.GitHubCredentialLease{
				TeamID: grant.SourceControl.TeamID, SourceControlID: grant.SourceControl.ID,
				ConnectionGeneration: 1, RepositoryID: grant.Repository.ID, GrantVersion: 1,
				AuthorizationID: grant.Snapshot().AuthorizationID, AuthorizationGeneration: 1,
				InstallationID: f.a.SourceControl.GetInstallationID(), Status: status,
				Token: dbtype.EncryptedString("lease-" + status + "-" + grant.SourceControl.UserID), ExpiresAt: time.Now().Add(time.Hour),
			}
			require.NoError(t, db.Create(&lease).Error)
			f.leases = append(f.leases, lease)
		}
	}
	// Existing completed deployment history must survive sponsorship removal.
	require.NoError(t, db.Exec("CREATE TABLE deployments (id TEXT PRIMARY KEY, team_id TEXT NOT NULL, status TEXT NOT NULL, commit_hash TEXT NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO deployments VALUES (?, ?, ?, ?)", "history-a", "team-a", "success", "existing-commit").Error)
	return f
}

func TestRevokeDeletedGitHubSponsorIsolatesLocalLogin(t *testing.T) {
	f := newSponsorDeletionFixture(t)
	var beforeB gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&beforeB, "id = ?", f.b.Snapshot().AuthorizationID).Error)
	var connectionB gitmodels.SourceControl
	require.NoError(t, f.db.First(&connectionB, "id = ?", f.b.SourceControl.ID).Error)
	var installation gitmodels.GitHubInstallation
	require.NoError(t, f.db.First(&installation, "id = ?", *f.a.SourceControl.GitHubInstallationID).Error)

	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
		return revokeDeletedGitHubSponsor(context.Background(), tx, f.a.SourceControl.UserID)
	}))
	var revoked gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&revoked, "id = ?", f.a.Snapshot().AuthorizationID).Error)
	require.NotNil(t, revoked.RevokedAt)
	require.Equal(t, uint64(2), revoked.Generation)
	require.Empty(t, revoked.AccessToken)
	require.Empty(t, revoked.RefreshToken)
	var changed gitmodels.SourceControl
	require.NoError(t, f.db.First(&changed, "id = ?", f.a.SourceControl.ID).Error)
	require.Equal(t, "reconnect_required", changed.Status)
	require.Equal(t, uint64(2), changed.Generation)
	var unchangedB gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&unchangedB, "id = ?", beforeB.ID).Error)
	require.Equal(t, beforeB, unchangedB)
	var unchangedConnection gitmodels.SourceControl
	require.NoError(t, f.db.First(&unchangedConnection, "id = ?", connectionB.ID).Error)
	require.Equal(t, connectionB, unchangedConnection)
	var unchangedInstallation gitmodels.GitHubInstallation
	require.NoError(t, f.db.First(&unchangedInstallation, "id = ?", installation.ID).Error)
	require.Equal(t, installation, unchangedInstallation)
	for _, before := range f.leases {
		var after gitmodels.GitHubCredentialLease
		require.NoError(t, f.db.First(&after, "id = ?", before.ID).Error)
		expected := before.Status
		if before.AuthorizationID == revoked.ID && (expected == "issuing" || expected == "released") {
			expected = "revoke_pending"
		}
		require.Equal(t, expected, after.Status)
		require.Equal(t, before.Token, after.Token, "pending token must remain available to the revocation worker")
	}
	for i, id := range f.manifestIDs {
		var manifest gitmodels.GitHubCleanupManifest
		require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
		if i == 0 {
			require.Equal(t, "pending", manifest.Status)
			require.Equal(t, uint64(2), manifest.ConnectionGeneration)
			require.NotNil(t, manifest.ExpiresAt)
		} else {
			require.Equal(t, "registered", manifest.Status)
			require.Equal(t, uint64(1), manifest.ConnectionGeneration)
			require.Nil(t, manifest.ExpiresAt)
		}
	}
	var saved gitmodels.SourceControlRepository
	require.NoError(t, f.db.First(&saved, "id = ?", f.a.Repository.ID).Error)
	require.True(t, saved.Selected, "saved intent remains for explicit reconnect review")
	require.Equal(t, f.a.Repository.FullName, saved.FullName)
	var count int64
	require.NoError(t, f.db.Table("deployments").Where("id = ? AND status = ? AND commit_hash = ?", "history-a", "success", "existing-commit").Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err := gitaccess.NewResolver(f.db, nil).Resolve(context.Background(), gitaccess.Request{TeamID: "team-a", SourceControlID: f.a.SourceControl.ID, RepositoryID: f.a.Snapshot().RepositoryID, Operation: gitaccess.Clone})
	require.Error(t, err)
	_, err = gitaccess.NewResolver(f.db, nil).Resolve(context.Background(), gitaccess.Request{TeamID: "team-b", SourceControlID: f.b.SourceControl.ID, RepositoryID: f.b.Snapshot().RepositoryID, Operation: gitaccess.Clone})
	require.NoError(t, err)
}

func TestRevokeDeletedGitHubSponsorRollsBackWithAccountDeletion(t *testing.T) {
	f := newSponsorDeletionFixture(t)
	deleteFailure := errors.New("account deletion failed")
	err := f.db.Transaction(func(tx *gorm.DB) error {
		if err := revokeDeletedGitHubSponsor(context.Background(), tx, f.a.SourceControl.UserID); err != nil {
			return err
		}
		return deleteFailure
	})
	require.ErrorIs(t, err, deleteFailure)
	var authorization gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&authorization, "id = ?", f.a.Snapshot().AuthorizationID).Error)
	require.Nil(t, authorization.RevokedAt)
	require.Equal(t, uint64(1), authorization.Generation)
	require.NotEmpty(t, authorization.AccessToken)
	require.NotEmpty(t, authorization.RefreshToken)
	var connection gitmodels.SourceControl
	require.NoError(t, f.db.First(&connection, "id = ?", f.a.SourceControl.ID).Error)
	require.Equal(t, "active", connection.Status)
	require.Equal(t, uint64(1), connection.Generation)
	for _, id := range f.manifestIDs {
		var manifest gitmodels.GitHubCleanupManifest
		require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
		require.Equal(t, "registered", manifest.Status)
		require.Nil(t, manifest.ExpiresAt)
	}
	for _, before := range f.leases {
		var after gitmodels.GitHubCredentialLease
		require.NoError(t, f.db.First(&after, "id = ?", before.ID).Error)
		require.Equal(t, before.Status, after.Status)
	}
}

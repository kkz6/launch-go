package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/security"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type githubLifecycleFixture struct {
	db             *gorm.DB
	service        *UserService
	userA          models.User
	personalTeamID string
	sharedTeamID   string
	a, b           *gitaccess.RepositoryAccess
	leases         []gitmodels.GitHubCredentialLease
}

func newGitHubLifecycleFixture(t *testing.T) githubLifecycleFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.TeamInvitation{}, &gitmodels.GitHubCleanupManifest{}))
	password, err := security.HashPassword("correct-password")
	require.NoError(t, err)
	f := githubLifecycleFixture{db: db, service: NewUserService(repositories.NewRegistry(db))}
	f.userA = models.User{Name: "A", Email: "a@example.com", Password: password}
	b := models.User{Name: "B", Email: "b@example.com", Password: password}
	owner := models.User{Name: "Workspace owner", Email: "owner@example.com", Password: password}
	for _, user := range []*models.User{&f.userA, &b, &owner} {
		require.NoError(t, db.Create(user).Error)
	}
	personal := models.Team{UserID: f.userA.ID, Name: "A personal", PersonalTeam: true}
	shared := models.Team{UserID: owner.ID, Name: "Surviving shared workspace"}
	teamB := models.Team{UserID: b.ID, Name: "B personal", PersonalTeam: true}
	for _, team := range []*models.Team{&personal, &shared, &teamB} {
		require.NoError(t, db.Create(team).Error)
	}
	f.personalTeamID, f.sharedTeamID = personal.ID, shared.ID
	role := "admin"
	require.NoError(t, db.Create(&models.TeamMember{TeamID: shared.ID, UserID: f.userA.ID, Role: &role}).Error)
	f.a = githubfixture.SelectedRepository(t, db, shared.ID, "shared/frontend", map[string]any{})
	f.b = githubfixture.SelectedRepository(t, db, teamB.ID, "shared/backend", map[string]any{})
	// A sponsors a workspace owned by someone else; B independently uses the
	// same upstream identity and installation from another launchctl login.
	require.NoError(t, db.Model(f.a.SourceControl).Update("user_id", f.userA.ID).Error)
	require.NoError(t, db.Model(&gitmodels.GitHubAuthorization{}).Where("id = ?", f.a.Snapshot().AuthorizationID).Update("user_id", f.userA.ID).Error)
	require.NoError(t, db.Model(f.b.SourceControl).Updates(map[string]any{"installation_id": f.a.SourceControl.InstallationID, "git_hub_installation_id": f.a.SourceControl.GitHubInstallationID}).Error)
	for _, grant := range []*gitaccess.RepositoryAccess{f.a, f.b} {
		require.NoError(t, db.Model(&gitmodels.GitHubAuthorization{}).Where("id = ?", grant.Snapshot().AuthorizationID).Updates(map[string]any{
			"git_hub_user_id": "same-upstream-user", "access_token": dbtype.EncryptedString("access-" + grant.SourceControl.ID), "refresh_token": dbtype.EncryptedString("refresh-" + grant.SourceControl.ID),
		}).Error)
		_, err := gitaccess.RegisterCleanup(context.Background(), db, grant.SourceControl.ID, grant.Snapshot().RepositoryID, grant.Snapshot(), []gitaccess.Resource{{Kind: "secret", Name: "LAUNCH_WORKLOAD_TOKEN"}})
		require.NoError(t, err)
		lease := gitmodels.GitHubCredentialLease{TeamID: grant.SourceControl.TeamID, SourceControlID: grant.SourceControl.ID, ConnectionGeneration: 1, RepositoryID: grant.Repository.ID, GrantVersion: 1, AuthorizationID: grant.Snapshot().AuthorizationID, AuthorizationGeneration: 1, InstallationID: f.a.SourceControl.GetInstallationID(), Token: dbtype.EncryptedString("lease-" + grant.SourceControl.ID), ExpiresAt: time.Now().Add(time.Hour), Status: "released"}
		require.NoError(t, db.Create(&lease).Error)
		f.leases = append(f.leases, lease)
	}
	require.NoError(t, db.Exec("CREATE TABLE deployments (id TEXT PRIMARY KEY, team_id TEXT NOT NULL, status TEXT NOT NULL, commit_hash TEXT NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO deployments VALUES (?, ?, ?, ?)", "shared-history", shared.ID, "success", "retained-commit").Error)
	return f
}

func TestDeleteAccountRevokesOnlyItsGitHubSponsorship(t *testing.T) {
	f := newGitHubLifecycleFixture(t)
	ctx := context.Background()
	var beforeB gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&beforeB, "id = ?", f.b.Snapshot().AuthorizationID).Error)
	require.Error(t, f.service.DeleteAccount(ctx, f.userA.ID, "wrong-password"))
	var untouched gitmodels.SourceControl
	require.NoError(t, f.db.First(&untouched, "id = ?", f.a.SourceControl.ID).Error)
	require.Equal(t, "active", untouched.Status)
	require.NoError(t, f.service.DeleteAccount(ctx, f.userA.ID, "correct-password"))
	var count int64
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.userA.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, f.db.Model(&models.TeamMember{}).Where("user_id = ?", f.userA.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, f.db.Model(&models.Team{}).Where("id = ?", f.personalTeamID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, f.db.Model(&models.Team{}).Where("id = ?", f.sharedTeamID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var authorization gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&authorization, "id = ?", f.a.Snapshot().AuthorizationID).Error)
	require.NotNil(t, authorization.RevokedAt)
	require.Equal(t, uint64(2), authorization.Generation)
	require.Empty(t, authorization.AccessToken)
	require.Empty(t, authorization.RefreshToken)
	var connection gitmodels.SourceControl
	require.NoError(t, f.db.First(&connection, "id = ?", f.a.SourceControl.ID).Error)
	require.Equal(t, "reconnect_required", connection.Status)
	require.Equal(t, uint64(2), connection.Generation)
	var unchangedB gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&unchangedB, "id = ?", beforeB.ID).Error)
	require.Equal(t, beforeB, unchangedB)
	for i, before := range f.leases {
		var lease gitmodels.GitHubCredentialLease
		require.NoError(t, f.db.First(&lease, "id = ?", before.ID).Error)
		if i == 0 {
			require.Equal(t, "revoke_pending", lease.Status)
		} else {
			require.Equal(t, "released", lease.Status)
		}
		require.Equal(t, before.Token, lease.Token)
	}
	var repo gitmodels.SourceControlRepository
	require.NoError(t, f.db.First(&repo, "id = ?", f.a.Repository.ID).Error)
	require.True(t, repo.Selected)
	require.Equal(t, f.a.Repository.FullName, repo.FullName)
	require.NoError(t, f.db.Table("deployments").Where("id = ? AND team_id = ? AND status = ? AND commit_hash = ?", "shared-history", f.sharedTeamID, "success", "retained-commit").Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err := gitaccess.NewResolver(f.db, nil).Resolve(ctx, gitaccess.Request{TeamID: f.sharedTeamID, SourceControlID: f.a.SourceControl.ID, RepositoryID: f.a.Snapshot().RepositoryID, Operation: gitaccess.Clone})
	require.Error(t, err)
	_, err = gitaccess.NewResolver(f.db, nil).Resolve(ctx, gitaccess.Request{TeamID: f.b.SourceControl.TeamID, SourceControlID: f.b.SourceControl.ID, RepositoryID: f.b.Snapshot().RepositoryID, Operation: gitaccess.Clone})
	require.NoError(t, err)
}

func TestDeleteAccountRollsBackGitHubRevocationWhenDeletionFails(t *testing.T) {
	f := newGitHubLifecycleFixture(t)
	deleteFailure := errors.New("injected user deletion failure")
	require.NoError(t, f.db.Callback().Delete().Before("gorm:delete").Register("test:fail_final_user_delete", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(deleteFailure)
		}
	}))
	require.ErrorIs(t, f.service.DeleteAccount(context.Background(), f.userA.ID, "correct-password"), deleteFailure)
	var authorization gitmodels.GitHubAuthorization
	require.NoError(t, f.db.First(&authorization, "id = ?", f.a.Snapshot().AuthorizationID).Error)
	require.Nil(t, authorization.RevokedAt)
	require.Equal(t, uint64(1), authorization.Generation)
	require.NotEmpty(t, authorization.AccessToken)
	var connection gitmodels.SourceControl
	require.NoError(t, f.db.First(&connection, "id = ?", f.a.SourceControl.ID).Error)
	require.Equal(t, "active", connection.Status)
	require.Equal(t, uint64(1), connection.Generation)
	var lease gitmodels.GitHubCredentialLease
	require.NoError(t, f.db.First(&lease, "id = ?", f.leases[0].ID).Error)
	require.Equal(t, "released", lease.Status)
	var count int64
	require.NoError(t, f.db.Model(&models.TeamMember{}).Where("user_id = ? AND team_id = ?", f.userA.ID, f.sharedTeamID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, f.db.Model(&models.Team{}).Where("id = ?", f.personalTeamID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.userA.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

package services

import (
	"context"
	"errors"
	"testing"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGitHubBoundTeamCannotTransferItsRepositoryGrants(t *testing.T) {
	db := newTeamServiceTestDB(t)
	service, source, destination := seedTeamDeletion(t, db)
	grant := githubfixture.SelectedRepository(t, db, source.ID, "org/private", map[string]any{})
	_, err := service.DeleteTeam(context.Background(), "owner", source.ID, destination.ID)
	require.ErrorContains(t, err, "GitHub connections")
	var sc gitmodels.SourceControl
	require.NoError(t, db.First(&sc, "id = ?", grant.SourceControl.ID).Error)
	require.Equal(t, source.ID, sc.TeamID)
	var count int64
	require.NoError(t, db.Table("teams").Where("id = ?", source.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSponsorRemovalInvalidatesOnlyTheirConnectionsInThatTeam(t *testing.T) {
	db := newTeamServiceTestDB(t)
	_, source, destination := seedTeamDeletion(t, db)
	own := githubfixture.SelectedRepository(t, db, source.ID, "org/one", map[string]any{})
	other := githubfixture.SelectedRepository(t, db, destination.ID, "org/two", map[string]any{})
	require.NoError(t, db.AutoMigrate(&gitmodels.GitHubCleanupManifest{}))
	require.NoError(t, db.Model(&gitmodels.GitHubAuthorization{}).Where("id IN ?", []string{*own.SourceControl.GitHubAuthorizationID, *other.SourceControl.GitHubAuthorizationID}).Update("user_id", "member").Error)
	require.NoError(t, repositories.NewTeamMemberRepository(db).AddUser(context.Background(), destination.ID, "member", "admin"))
	lease := gitmodels.GitHubCredentialLease{TeamID: source.ID, SourceControlID: own.SourceControl.ID, Status: "released"}
	require.NoError(t, db.Create(&lease).Error)
	require.NoError(t, repositories.NewTeamMemberRepository(db).RemoveUser(context.Background(), source.ID, "member"))
	var sc gitmodels.SourceControl
	require.NoError(t, db.First(&sc, "id = ?", own.SourceControl.ID).Error)
	require.Equal(t, "reconnect_required", sc.Status)
	require.Equal(t, uint64(2), sc.Generation)
	require.NoError(t, db.First(&lease, "id = ?", lease.ID).Error)
	require.Equal(t, "revoke_pending", lease.Status)
	var unaffected gitmodels.SourceControl
	require.NoError(t, db.First(&unaffected, "id = ?", other.SourceControl.ID).Error)
	require.Equal(t, "active", unaffected.Status)
	require.Equal(t, uint64(1), unaffected.Generation)
}

func TestSponsorMembershipAndGrantsRollbackTogether(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "demotion"
		if remove {
			name = "removal"
		}
		t.Run(name, func(t *testing.T) {
			db := newTeamServiceTestDB(t)
			_, source, _ := seedTeamDeletion(t, db)
			grant := githubfixture.SelectedRepository(t, db, source.ID, "org/one", map[string]any{})
			require.NoError(t, db.Model(&gitmodels.GitHubAuthorization{}).Where("id = ?", *grant.SourceControl.GitHubAuthorizationID).Update("user_id", "member").Error)
			require.NoError(t, db.Table("team_user").Where("team_id = ? AND user_id = ?", source.ID, "member").Update("role", "admin").Error)
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail-grant-invalidation", func(tx *gorm.DB) {
				if tx.Statement.Table == "source_controls" {
					tx.AddError(errors.New("grant update unavailable"))
				}
			}))
			repo := repositories.NewTeamMemberRepository(db)
			var err error
			if remove {
				err = repo.RemoveUser(context.Background(), source.ID, "member")
			} else {
				err = repo.UpdateRole(context.Background(), source.ID, "member", "viewer")
			}
			require.ErrorContains(t, err, "grant update unavailable")
			var member authmodels.TeamMember
			require.NoError(t, db.Where("team_id = ? AND user_id = ?", source.ID, "member").First(&member).Error)
			require.Equal(t, "admin", member.GetRole())
			var sc gitmodels.SourceControl
			require.NoError(t, db.First(&sc, "id = ?", grant.SourceControl.ID).Error)
			require.Equal(t, "active", sc.Status)
			require.Equal(t, uint64(1), sc.Generation)
			require.NoError(t, db.Callback().Update().Remove("test:fail-grant-invalidation"))
			if remove {
				err = repo.RemoveUser(context.Background(), source.ID, "member")
			} else {
				err = repo.UpdateRole(context.Background(), source.ID, "member", "viewer")
			}
			require.NoError(t, err)
			require.NoError(t, db.First(&sc, "id = ?", grant.SourceControl.ID).Error)
			require.Equal(t, "reconnect_required", sc.Status)
			require.Equal(t, uint64(2), sc.Generation)
			if remove {
				require.ErrorIs(t, db.Where("team_id = ? AND user_id = ?", source.ID, "member").First(&member).Error, gorm.ErrRecordNotFound)
			} else {
				require.NoError(t, db.Where("team_id = ? AND user_id = ?", source.ID, "member").First(&member).Error)
				require.Equal(t, "viewer", member.GetRole())
			}
		})
	}
}

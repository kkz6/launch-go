package handlers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

type roleResponseRegistry struct{ contracts.RepositoryRegistry }

func (r roleResponseRegistry) IsUserAdmin(context.Context, string) bool      { return false }
func (r roleResponseRegistry) IsTeamSubscribed(context.Context, string) bool { return false }

func TestAccountResponsesFollowSelectedTeamRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.Session{}))
	hash, err := security.HashPassword("correct-horse")
	require.NoError(t, err)
	user := &models.User{Name: "User", Email: "user@example.test", Password: hash}
	require.NoError(t, db.Create(user).Error)
	ownerTeam := &models.Team{BaseModel: basemodels.BaseModel{ID: "01h00000000000000000000001"}, UserID: user.ID, Name: "Owned"}
	editorTeam := &models.Team{BaseModel: basemodels.BaseModel{ID: "01h00000000000000000000002"}, UserID: "other", Name: "Editable"}
	memberTeam := &models.Team{BaseModel: basemodels.BaseModel{ID: "01h00000000000000000000003"}, UserID: "other", Name: "View only"}
	for _, team := range []*models.Team{ownerTeam, editorTeam, memberTeam} {
		require.NoError(t, db.Create(team).Error)
	}
	for role, team := range map[string]*models.Team{"editor": editorTeam, "member": memberTeam} {
		require.NoError(t, db.Create(&models.TeamMember{TeamID: team.ID, UserID: user.ID, Role: &role}).Error)
	}
	// Owners do not need a pivot row to retain their owner permissions.
	require.NoError(t, db.Model(user).Update("current_team_id", ownerTeam.ID).Error)
	logger := zerolog.Nop()
	store := newMockCache()
	service, err := services.NewService(roleResponseRegistry{repositories.NewRegistry(db)}, testConfig(), &logger, nil, store)
	require.NoError(t, err)
	service.SetMembershipCache(launchcache.NewTeamMembershipCache(store, db))

	login, err := service.Auth.Login(context.Background(), &dto.LoginRequest{Email: user.Email, Password: "correct-horse"})
	require.NoError(t, err)
	require.NotNil(t, login.AuthResponse)
	require.NotNil(t, login.AuthResponse.User.Role)
	require.Equal(t, "owner", *login.AuthResponse.User.Role)
	refreshToken := login.AuthResponse.RefreshToken

	for _, selected := range []struct {
		team *models.Team
		role string
	}{{memberTeam, "member"}, {editorTeam, "editor"}, {ownerTeam, "owner"}} {
		t.Run(selected.role, func(t *testing.T) {
			switched, err := service.Team.SwitchTeam(context.Background(), user.ID, selected.team.ID)
			require.NoError(t, err)
			current, err := service.User.GetUser(context.Background(), user.ID)
			require.NoError(t, err)
			for _, account := range []*models.User{switched, current} {
				body := service.UserResponse(context.Background(), account)
				require.NotNil(t, body.Role)
				require.Equal(t, selected.role, *body.Role)
				require.Equal(t, selected.team.ID, *body.CurrentTeamID)
				require.Equal(t, selected.team.ID, body.CurrentTeam.ID)
			}
			refreshed, err := service.Auth.RefreshToken(context.Background(), refreshToken)
			require.NoError(t, err)
			require.NotNil(t, refreshed.User.Role)
			require.Equal(t, selected.role, *refreshed.User.Role)
			refreshToken = refreshed.RefreshToken
		})
	}

	missingTeam := "missing-team"
	user.CurrentTeamID = &missingTeam
	require.Nil(t, service.UserResponse(context.Background(), user).Role, "unknown membership must not grant a role")
	user.CurrentTeamID = nil
	require.Nil(t, service.UserResponse(context.Background(), user).Role)
}

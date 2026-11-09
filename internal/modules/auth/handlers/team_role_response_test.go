package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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

	app := fiber.New(fiber.Config{DisableStartupMessage: true, ErrorHandler: fiberutil.NewErrorHandler()})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(fiberutil.KeyUserID, user.ID)
		return c.Next()
	})
	app.Post("/teams/:teamId/switch", handlers.NewTeamHandler(service).SwitchTeamByID)
	app.Get("/user", handlers.NewUserHandler(service).User)
	// A real listener initializes fasthttp's request context cancellation channel.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-done)
	})
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://" + listener.Addr().String()

	for _, selected := range []struct {
		team *models.Team
		role string
	}{{memberTeam, "member"}, {editorTeam, "editor"}, {ownerTeam, "owner"}} {
		t.Run(selected.role, func(t *testing.T) {
			for _, route := range []struct{ method, path string }{
				{http.MethodPost, "/teams/" + selected.team.ID + "/switch"},
				{http.MethodGet, "/user"},
			} {
				request, err := http.NewRequest(route.method, baseURL+route.path, nil)
				require.NoError(t, err)
				response, err := client.Do(request)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				var body dto.UserResponse
				require.NoError(t, json.Unmarshal(parseResponse(response).Data, &body))
				require.NoError(t, response.Body.Close())
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

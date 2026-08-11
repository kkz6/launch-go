package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type handlerTransferResource struct {
	ID     uint   `gorm:"primaryKey"`
	TeamID string `gorm:"column:team_id"`
}

func TestTeamHandlerDeleteTransfersResources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{}, &models.Team{}, &models.TeamMember{}, &models.TeamInvitation{}, &handlerTransferResource{},
	))
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(owner).Error)
	source := &models.Team{BaseModel: basemodels.BaseModel{ID: "01h00000000000000000000001"}, UserID: owner.ID, Name: "Source"}
	destination := &models.Team{BaseModel: basemodels.BaseModel{ID: "01h00000000000000000000002"}, UserID: owner.ID, Name: "Destination", PersonalTeam: true}
	require.NoError(t, db.Create(source).Error)
	require.NoError(t, db.Create(destination).Error)
	require.NoError(t, db.Create(&handlerTransferResource{TeamID: source.ID}).Error)

	service := &services.Service{Team: services.NewTeamService(repositories.NewRegistry(db))}
	handler := handlers.NewTeamHandler(service)
	app := newTestAppWithValidation()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", owner.ID)
		return c.Next()
	})
	app.Delete("/teams/:teamId", fiberutil.Validate(handler.DeleteTeam))

	response, err := app.Test(makeJSONRequest(http.MethodDelete, "/teams/"+source.ID, map[string]string{
		"transfer_to_team_id": destination.ID,
	}), testTimeout)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	parsed := parseResponse(response)
	require.True(t, parsed.Success)
	require.Equal(t, "Team deleted and resources transferred successfully", parsed.Message)

	var resource handlerTransferResource
	require.NoError(t, db.First(&resource).Error)
	require.Equal(t, destination.ID, resource.TeamID)

	response, err = app.Test(makeJSONRequest(http.MethodDelete, "/teams/not-valid", map[string]string{
		"transfer_to_team_id": destination.ID,
	}), testTimeout)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, response.StatusCode)

	response, err = app.Test(makeJSONRequest(http.MethodDelete, "/teams/"+destination.ID, map[string]string{
		"transfer_to_team_id": destination.ID,
	}), testTimeout)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)

	unauthenticated := newTestAppWithValidation()
	unauthenticated.Delete("/teams/:teamId", fiberutil.Validate(handler.DeleteTeam))
	response, err = unauthenticated.Test(makeJSONRequest(http.MethodDelete, "/teams/"+destination.ID, map[string]string{
		"transfer_to_team_id": destination.ID,
	}), testTimeout)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}

package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

func TestInvitationDetailsIdentifiesTheTeam(t *testing.T) {
	registry := newMockRegistry()
	team := &models.Team{Name: "Shared"}
	team.ID = "01TEAM123456"
	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "member@example.com",
		Team:   team,
	}
	invitation.ID = "invite"
	registry.teamInvitation.invitations[invitation.ID] = invitation
	registry.user.users["member"] = newTestUser("member", "Member", invitation.Email, "")

	service := &services.Service{
		TeamMember: services.NewTeamMemberService(registry, testConfig(), nil, nil),
	}
	handler := handlers.NewTeamMemberHandler(service)
	app := fiber.New()
	app.Get("/invitations/:invitationId", handler.GetInvitationDetails)

	response, err := app.Test(makeJSONRequest(http.MethodGet, "/invitations/invite", nil), testTimeout)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	parsed := parseResponse(response)
	var data struct {
		TeamID     string `json:"team_id"`
		TeamName   string `json:"team_name"`
		UserExists bool   `json:"user_exists"`
	}
	require.NoError(t, json.Unmarshal(parsed.Data, &data))
	require.Equal(t, team.ID, data.TeamID)
	require.Equal(t, team.Name, data.TeamName)
	require.True(t, data.UserExists)
}

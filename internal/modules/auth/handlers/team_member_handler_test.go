package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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

func TestInviteAndResendReportUnavailableEmailDelivery(t *testing.T) {
	registry := newMockRegistry()
	team := &models.Team{UserID: "owner", Name: "Shared"}
	team.ID = "team"
	registry.team.teams[team.ID] = team
	logger := zerolog.Nop()
	handler := handlers.NewTeamMemberHandler(&services.Service{
		TeamMember: services.NewTeamMemberService(registry, testConfig(), nil, &logger),
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "owner")
		return c.Next()
	})
	app.Post("/teams/:teamId/invitations", fiberctx.Validate(handler.InviteTeamMember))
	app.Post("/teams/:teamId/invitations/:invitationId/resend", handler.ResendTeamInvitation)

	invitations := "/teams/team/invitations"
	for _, request := range []*http.Request{
		makeJSONRequest(http.MethodPost, invitations, map[string]string{"email": "invitee@example.com", "role": "member"}),
		makeJSONRequest(http.MethodPost, invitations+"/generated_invitation_id/resend", nil),
	} {
		response, err := app.Test(request, testTimeout)
		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
		var payload fiberctx.ErrorResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
		require.Equal(t, "team.invitation_email_failed", payload.Code)
		require.False(t, payload.Success)
		require.NoError(t, response.Body.Close())
	}
}

package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TeamHandler handles team management HTTP requests
type TeamHandler struct {
	BaseHandler
}

// NewTeamHandler creates a new TeamHandler instance
func NewTeamHandler(service *services.Service) *TeamHandler {
	return &TeamHandler{BaseHandler: NewBaseHandler(service)}
}

// CreateTeam creates a new team
func (h *TeamHandler) CreateTeam(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateTeamRequest](c)
	if err != nil {
		return err
	}

	team, err := h.Service().CreateTeam(c.Context(), userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Team created successfully", dto.ToTeamResponse(*team))
}

// GetTeam retrieves a team with details
func (h *TeamHandler) GetTeam(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	team, members, invitations, err := h.Service().GetTeamWithDetails(c.Context(), teamID)
	if err != nil {
		return response.HandleError(c, err)
	}

	if team == nil {
		return response.NotFound(c, response.MsgTeamNotFound)
	}

	return response.OK(c, "Team retrieved", dto.ToTeamDetailResponse(team, members, invitations, userID))
}

// UpdateTeam updates a team
func (h *TeamHandler) UpdateTeam(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateTeamRequest](c)
	if err != nil {
		return err
	}

	team, err := h.Service().UpdateTeam(c.Context(), userID, teamID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Team updated successfully", dto.ToTeamResponse(*team))
}

// DeleteTeam deletes a team
func (h *TeamHandler) DeleteTeam(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	if err := h.Service().DeleteTeam(c.Context(), userID, teamID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Team deleted successfully", nil)
}

// GetUserTeams retrieves all teams for the current user
func (h *TeamHandler) GetUserTeams(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teams, err := h.Service().GetUserTeams(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Teams retrieved", dto.ToTeamsResponseForUser(teams, userID))
}

// SwitchTeam switches the user's current team (from request body)
// After switching, the frontend should use the returned team ID in the X-Team-ID header
// for all subsequent API requests that require team context.
func (h *TeamHandler) SwitchTeam(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.SwitchTeamRequest](c)
	if err != nil {
		return err
	}

	user, err := h.Service().SwitchTeam(c.Context(), userID, req.TeamID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Team switched. Use X-Team-ID header for subsequent requests.", dto.ToUserResponse(user))
}

// SwitchTeamByID switches the user's current team using URL parameter
// After switching, the frontend should use the returned team ID in the X-Team-ID header
// for all subsequent API requests that require team context.
func (h *TeamHandler) SwitchTeamByID(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	user, err := h.Service().SwitchTeam(c.Context(), userID, teamID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Team switched. Use X-Team-ID header for subsequent requests.", dto.ToUserResponse(user))
}

package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
func (h *TeamHandler) CreateTeam(c *fiber.Ctx, req *dto.CreateTeamRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	team, err := h.Service().Team.CreateTeam(c.Context(), userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Team created successfully", dto.ToTeamResponse(*team))
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

	team, members, invitations, err := h.Service().Team.GetTeamWithDetails(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	if team == nil {
		return fiberctx.RespondNotFound(c, "Team not found")
	}

	return fiberctx.OK(c, "Team retrieved", dto.ToTeamDetailResponse(team, members, invitations, userID))
}

// UpdateTeam updates a team
func (h *TeamHandler) UpdateTeam(c *fiber.Ctx, req *dto.UpdateTeamRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	team, err := h.Service().Team.UpdateTeam(c.Context(), userID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Team updated successfully", dto.ToTeamResponse(*team))
}

// DeleteTeam deletes a team
func (h *TeamHandler) DeleteTeam(c *fiber.Ctx, req *dto.DeleteTeamRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	destination, err := h.Service().Team.DeleteTeam(c.UserContext(), userID, teamID, req.TransferToTeamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Team deleted and resources transferred successfully", dto.ToTeamResponseForUser(*destination, userID))
}

// GetUserTeams retrieves all teams for the current user
func (h *TeamHandler) GetUserTeams(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teams, err := h.Service().Team.GetUserTeams(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Teams retrieved", dto.ToTeamsResponseForUser(teams, userID))
}

// SwitchTeam switches the user's current team (from request body)
func (h *TeamHandler) SwitchTeam(c *fiber.Ctx, req *dto.SwitchTeamRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	return h.handleSwitchTeam(c, userID, req.TeamID)
}

// SwitchTeamByID switches the user's current team using URL parameter
func (h *TeamHandler) SwitchTeamByID(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.GetTeamIDParam(c)
	if err != nil {
		return err
	}

	return h.handleSwitchTeam(c, userID, teamID)
}

func (h *TeamHandler) handleSwitchTeam(c *fiber.Ctx, userID, teamID string) error {
	user, err := h.Service().Team.SwitchTeam(c.Context(), userID, teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	isSubscribed := h.Service().IsUserSubscribed(c.Context(), user)

	return fiberctx.OK(c, "Team switched. Use X-Team-ID header for subsequent requests.", dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded))
}

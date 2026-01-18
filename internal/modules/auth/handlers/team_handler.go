package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// TeamHandler handles team management HTTP requests
type TeamHandler struct {
	service *services.Service
}

// NewTeamHandler creates a new TeamHandler instance
func NewTeamHandler(service *services.Service) *TeamHandler {
	return &TeamHandler{service: service}
}

// CreateTeam creates a new team
func (h *TeamHandler) CreateTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.CreateTeamRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	team, err := h.service.CreateTeam(c.Context(), userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Team created successfully", dto.ToTeamResponse(*team))
}

// GetTeam retrieves a team with details
func (h *TeamHandler) GetTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	team, members, invitations, err := h.service.GetTeamWithDetails(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	if team == nil {
		return response.NotFound(c, "Team not found")
	}

	return response.OK(c, "Team retrieved", dto.ToTeamDetailResponse(team, members, invitations, userID))
}

// UpdateTeam updates a team
func (h *TeamHandler) UpdateTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	var req dto.UpdateTeamRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	team, err := h.service.UpdateTeam(c.Context(), userID, teamID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team updated successfully", dto.ToTeamResponse(*team))
}

// DeleteTeam deletes a team
func (h *TeamHandler) DeleteTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	if err := h.service.DeleteTeam(c.Context(), userID, teamID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team deleted successfully", nil)
}

// GetUserTeams retrieves all teams for the current user
func (h *TeamHandler) GetUserTeams(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	teams, err := h.service.GetUserTeams(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Teams retrieved", dto.ToTeamsResponse(teams))
}

// SwitchTeam switches the user's current team (from request body)
// After switching, the frontend should use the returned team ID in the X-Team-ID header
// for all subsequent API requests that require team context.
func (h *TeamHandler) SwitchTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.SwitchTeamRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	user, err := h.service.SwitchTeam(c.Context(), userID, req.TeamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team switched. Use X-Team-ID header for subsequent requests.", dto.ToUserResponse(user))
}

// SwitchTeamByID switches the user's current team using URL parameter
// After switching, the frontend should use the returned team ID in the X-Team-ID header
// for all subsequent API requests that require team context.
func (h *TeamHandler) SwitchTeamByID(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	if teamID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Team ID is required")
	}

	user, err := h.service.SwitchTeam(c.Context(), userID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team switched. Use X-Team-ID header for subsequent requests.", dto.ToUserResponse(user))
}

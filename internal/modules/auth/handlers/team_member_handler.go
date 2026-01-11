package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// TeamMemberHandler handles team member management HTTP requests
type TeamMemberHandler struct {
	service *services.Service
}

// NewTeamMemberHandler creates a new TeamMemberHandler instance
func NewTeamMemberHandler(service *services.Service) *TeamMemberHandler {
	return &TeamMemberHandler{service: service}
}

// InviteTeamMember invites a user to a team
func (h *TeamMemberHandler) InviteTeamMember(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	var req dto.InviteTeamMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.InviteTeamMember(c.Context(), userID, teamID, &req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Invitation sent successfully", nil)
}

// AcceptTeamInvitation accepts a team invitation
func (h *TeamMemberHandler) AcceptTeamInvitation(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	invitationID := c.Params("invitationId")

	if err := h.service.AcceptTeamInvitation(c.Context(), userID, invitationID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitation accepted", nil)
}

// CancelTeamInvitation cancels a team invitation
func (h *TeamMemberHandler) CancelTeamInvitation(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	invitationID := c.Params("invitationId")

	if err := h.service.CancelTeamInvitation(c.Context(), userID, teamID, invitationID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitation cancelled", nil)
}

// UpdateTeamMemberRole updates a team member's role
func (h *TeamMemberHandler) UpdateTeamMemberRole(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	memberID := c.Params("memberId")

	var req dto.UpdateTeamMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.UpdateTeamMemberRole(c.Context(), userID, teamID, memberID, &req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Member role updated", nil)
}

// RemoveTeamMember removes a member from a team
func (h *TeamMemberHandler) RemoveTeamMember(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	memberID := c.Params("memberId")

	if err := h.service.RemoveTeamMember(c.Context(), userID, teamID, memberID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Member removed", nil)
}

// GetTeamMembers retrieves all members of a team
func (h *TeamMemberHandler) GetTeamMembers(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	members, err := h.service.GetTeamMembers(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Members retrieved", dto.ToTeamMembersResponse(members))
}

// GetTeamInvitations retrieves all invitations for a team
func (h *TeamMemberHandler) GetTeamInvitations(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	invitations, err := h.service.GetTeamInvitations(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitations retrieved", dto.ToTeamInvitationsResponse(invitations))
}

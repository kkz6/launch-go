package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// TeamMemberHandler handles team member management HTTP requests
type TeamMemberHandler struct {
	BaseHandler
}

// NewTeamMemberHandler creates a new TeamMemberHandler instance
func NewTeamMemberHandler(service *services.Service) *TeamMemberHandler {
	return &TeamMemberHandler{BaseHandler: NewBaseHandler(service)}
}

// InviteTeamMember invites a user to a team
func (h *TeamMemberHandler) InviteTeamMember(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	teamID := c.Params("teamId")

	req, err := fiberctx.MustParseAndValidate[dto.InviteTeamMemberRequest](c)
	if err != nil {
		return err
	}

	if err := h.Service().TeamMember.InviteTeamMember(c.Context(), userID, teamID, req); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Invitation sent successfully", nil)
}

// AcceptTeamInvitation accepts a team invitation
func (h *TeamMemberHandler) AcceptTeamInvitation(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	invitationID := c.Params("invitationId")

	if err := h.Service().TeamMember.AcceptTeamInvitation(c.Context(), userID, invitationID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Invitation accepted", nil)
}

// CancelTeamInvitation cancels a team invitation
func (h *TeamMemberHandler) CancelTeamInvitation(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	teamID := c.Params("teamId")
	invitationID := c.Params("invitationId")

	if err := h.Service().TeamMember.CancelTeamInvitation(c.Context(), userID, teamID, invitationID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Invitation cancelled", nil)
}

// UpdateTeamMemberRole updates a team member's role
func (h *TeamMemberHandler) UpdateTeamMemberRole(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	teamID := c.Params("teamId")
	targetUserID := c.Params("userId")

	req, err := fiberctx.MustParseAndValidate[dto.UpdateTeamMemberRequest](c)
	if err != nil {
		return err
	}

	if err := h.Service().TeamMember.UpdateTeamMemberRole(c.Context(), userID, teamID, targetUserID, req); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Member role updated", nil)
}

// RemoveTeamMember removes a member from a team
func (h *TeamMemberHandler) RemoveTeamMember(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	teamID := c.Params("teamId")
	targetUserID := c.Params("userId")

	if err := h.Service().TeamMember.RemoveTeamMember(c.Context(), userID, teamID, targetUserID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Member removed", nil)
}

// GetTeamMembers retrieves all members of a team (including owner)
func (h *TeamMemberHandler) GetTeamMembers(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	allMembers, err := h.Service().TeamMember.GetAllTeamMembers(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Members retrieved", allMembers)
}

// GetTeamInvitations retrieves all invitations for a team
func (h *TeamMemberHandler) GetTeamInvitations(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	invitations, err := h.Service().TeamMember.GetTeamInvitations(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Invitations retrieved", dto.ToTeamInvitationsResponse(invitations))
}

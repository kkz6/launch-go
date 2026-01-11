package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for authentication
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Authentication Handlers

// Register handles user registration
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	result, err := h.service.Register(c.Context(), &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Registration successful", result)
}

// Login handles user authentication
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	result, err := h.service.Login(c.Context(), &req)
	if err != nil {
		return response.Unauthorized(c, "Invalid credentials")
	}

	return response.OK(c, "Login successful", result)
}

// Logout handles user logout
func (h *Handler) Logout(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	if err := h.service.Logout(c.Context(), userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.OK(c, "Logged out successfully", nil)
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	result, err := h.service.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return response.Unauthorized(c, "Invalid refresh token")
	}

	return response.OK(c, "Token refreshed successfully", result)
}

// User Management Handlers

// User returns the current authenticated user
func (h *Handler) User(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	user, err := h.service.GetUser(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	if user == nil {
		return response.NotFound(c, "User not found")
	}

	return response.OK(c, "User retrieved", ToUserResponse(user))
}

// UpdateProfile updates the user's profile
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	user, err := h.service.UpdateProfile(c.Context(), userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Profile updated", ToUserResponse(user))
}

// ChangePassword changes the user's password
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.ChangePassword(c.Context(), userID, &req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Password changed successfully", nil)
}

// DeleteAccount deletes the user's account
func (h *Handler) DeleteAccount(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	if err := h.service.DeleteAccount(c.Context(), userID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Account deleted successfully", nil)
}

// Email Verification Handlers

// VerifyEmail verifies the user's email
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	userID := c.Params("id")
	hash := c.Params("hash")

	if err := h.service.VerifyEmail(c.Context(), userID, hash); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Email verified successfully", nil)
}

// ResendVerificationEmail resends the verification email
func (h *Handler) ResendVerificationEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	if err := h.service.ResendVerificationEmail(c.Context(), userID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Verification email sent", nil)
}

// Password Reset Handlers

// ForgotPassword initiates password reset
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	// Always return success to prevent email enumeration
	_ = h.service.SendPasswordResetLink(c.Context(), req.Email)

	return response.OK(c, "If an account with that email exists, a password reset link has been sent", nil)
}

// ResetPassword resets the user's password
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.ResetPassword(c.Context(), &req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Password reset successfully", nil)
}

// Two-Factor Authentication Handlers

// EnableTwoFactor initiates 2FA setup
func (h *Handler) EnableTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	result, err := h.service.EnableTwoFactor(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Two-factor authentication initiated", result)
}

// ConfirmTwoFactor confirms 2FA setup
func (h *Handler) ConfirmTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req ConfirmTwoFactorRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.ConfirmTwoFactor(c.Context(), userID, req.Code); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Two-factor authentication enabled", nil)
}

// DisableTwoFactor disables 2FA
func (h *Handler) DisableTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req EnableTwoFactorRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.service.DisableTwoFactor(c.Context(), userID, req.Password); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Two-factor authentication disabled", nil)
}

// TwoFactorChallenge verifies 2FA code during login
func (h *Handler) TwoFactorChallenge(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req TwoFactorChallengeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	code := req.Code
	if code == "" {
		code = req.RecoveryCode
	}

	valid, err := h.service.VerifyTwoFactor(c.Context(), userID, code)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	if !valid {
		return response.Unauthorized(c, "Invalid two-factor code")
	}

	return response.OK(c, "Two-factor authentication verified", nil)
}

// GetRecoveryCodes returns the user's recovery codes
func (h *Handler) GetRecoveryCodes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	codes, err := h.service.GetRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Recovery codes retrieved", fiber.Map{"recovery_codes": codes})
}

// RegenerateRecoveryCodes generates new recovery codes
func (h *Handler) RegenerateRecoveryCodes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	codes, err := h.service.RegenerateRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Recovery codes regenerated", fiber.Map{"recovery_codes": codes})
}

// Team Management Handlers

// CreateTeam creates a new team
func (h *Handler) CreateTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req CreateTeamRequest
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

	return response.Created(c, "Team created successfully", ToTeamResponse(*team))
}

// GetTeam retrieves a team
func (h *Handler) GetTeam(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	team, err := h.service.GetTeam(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	if team == nil {
		return response.NotFound(c, "Team not found")
	}

	return response.OK(c, "Team retrieved", ToTeamResponse(*team))
}

// UpdateTeam updates a team
func (h *Handler) UpdateTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	var req UpdateTeamRequest
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

	return response.OK(c, "Team updated successfully", ToTeamResponse(*team))
}

// DeleteTeam deletes a team
func (h *Handler) DeleteTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	if err := h.service.DeleteTeam(c.Context(), userID, teamID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team deleted successfully", nil)
}

// GetUserTeams retrieves all teams for the current user
func (h *Handler) GetUserTeams(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	teams, err := h.service.GetUserTeams(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Teams retrieved", ToTeamsResponse(teams))
}

// SwitchTeam switches the user's current team
func (h *Handler) SwitchTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	user, err := h.service.SwitchTeam(c.Context(), userID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team switched", ToUserResponse(user))
}

// Team Member Management Handlers

// InviteTeamMember invites a user to a team
func (h *Handler) InviteTeamMember(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	var req InviteTeamMemberRequest
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
func (h *Handler) AcceptTeamInvitation(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	invitationID := c.Params("invitationId")

	if err := h.service.AcceptTeamInvitation(c.Context(), userID, invitationID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitation accepted", nil)
}

// CancelTeamInvitation cancels a team invitation
func (h *Handler) CancelTeamInvitation(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	invitationID := c.Params("invitationId")

	if err := h.service.CancelTeamInvitation(c.Context(), userID, teamID, invitationID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitation cancelled", nil)
}

// UpdateTeamMemberRole updates a team member's role
func (h *Handler) UpdateTeamMemberRole(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	memberID := c.Params("memberId")

	var req UpdateTeamMemberRequest
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
func (h *Handler) RemoveTeamMember(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")
	memberID := c.Params("memberId")

	if err := h.service.RemoveTeamMember(c.Context(), userID, teamID, memberID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Member removed", nil)
}

// GetTeamMembers retrieves all members of a team
func (h *Handler) GetTeamMembers(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	members, err := h.service.GetTeamMembers(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Members retrieved", ToTeamMembersResponse(members))
}

// GetTeamInvitations retrieves all invitations for a team
func (h *Handler) GetTeamInvitations(c *fiber.Ctx) error {
	teamID := c.Params("teamId")

	invitations, err := h.service.GetTeamInvitations(c.Context(), teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Invitations retrieved", ToTeamInvitationsResponse(invitations))
}

// User Status Handler

// CheckUserStatus checks a user's status by email
func (h *Handler) CheckUserStatus(c *fiber.Ctx) error {
	var req CheckUserStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	status, err := h.service.CheckUserStatus(c.Context(), req.Email)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "User status retrieved", status)
}

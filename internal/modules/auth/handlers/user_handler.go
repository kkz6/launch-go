package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// UserHandler handles user management HTTP requests
type UserHandler struct {
	BaseHandler
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler(service *services.Service) *UserHandler {
	return &UserHandler{BaseHandler: NewBaseHandler(service)}
}

// User returns the current authenticated user
func (h *UserHandler) User(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	user, err := h.Service().User.GetUser(c.Context(), userID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "User not found")
	}

	if user == nil {
		return fiberctx.RespondNotFound(c, "User not found")
	}

	isSubscribed := h.Service().IsUserSubscribed(c.Context(), user)

	resp := dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded)
	if user.CurrentTeamID != nil {
		if role := h.Service().CurrentTeamRole(c.Context(), user.ID, *user.CurrentTeamID); role != "" {
			resp.Role = &role
		}
	}

	return fiberctx.OK(c, "User retrieved", resp)
}

// UpdateProfile updates the user's profile
func (h *UserHandler) UpdateProfile(c *fiber.Ctx, req *dto.UpdateProfileRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	user, err := h.Service().User.UpdateProfile(c.Context(), userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	isSubscribed := h.Service().IsUserSubscribed(c.Context(), user)

	return fiberctx.OK(c, "Profile updated", dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded))
}

// UpdateLocale updates the user's language preference. The new locale is
// applied before the response is serialized, so the confirmation itself uses
// the newly selected language.
func (h *UserHandler) UpdateLocale(c *fiber.Ctx, req *dto.UpdateLocaleRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	user, err := h.Service().User.UpdateLocale(c.Context(), userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	i18n.ApplyPreference(c, user.Locale)
	isSubscribed := h.Service().IsUserSubscribed(c.Context(), user)
	return fiberctx.OK(c, "Locale updated", dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded))
}

// ChangePassword changes the user's password
func (h *UserHandler) ChangePassword(c *fiber.Ctx, req *dto.ChangePasswordRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().User.ChangePassword(c.Context(), userID, req); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Password changed successfully", nil)
}

// DeleteAccount deletes the user's account
func (h *UserHandler) DeleteAccount(c *fiber.Ctx, req *dto.DeleteAccountRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().User.DeleteAccount(c.Context(), userID, req.Password); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Account deleted successfully", nil)
}

// CheckUserStatus checks a user's status by email
func (h *UserHandler) CheckUserStatus(c *fiber.Ctx, req *dto.CheckUserStatusRequest) error {
	status, err := h.Service().User.CheckUserStatus(c.Context(), req.Email)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "User status retrieved", status)
}

// ResetOnboarding resets the user's onboarding status
func (h *UserHandler) ResetOnboarding(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().SetOnboarded(c.Context(), userID, false); err != nil {
		return fiberctx.HandleError(c, err)
	}

	// Get updated user data
	user, err := h.Service().User.GetUser(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	isSubscribed := h.Service().IsUserSubscribed(c.Context(), user)

	return fiberctx.OK(c, "Onboarding reset successfully", dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded))
}

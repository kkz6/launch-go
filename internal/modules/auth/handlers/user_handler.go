package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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

	return fiberctx.OK(c, "User retrieved", dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded))
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

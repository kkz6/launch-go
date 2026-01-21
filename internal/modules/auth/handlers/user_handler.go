package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// UserHandler handles user management HTTP requests
type UserHandler struct {
	service *services.Service
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler(service *services.Service) *UserHandler {
	return &UserHandler{service: service}
}

// User returns the current authenticated user
func (h *UserHandler) User(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	user, err := h.service.GetUser(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, response.MsgUserNotFound)
	}

	if user == nil {
		return response.NotFound(c, response.MsgUserNotFound)
	}

	// Check if current team is subscribed or user is admin (admins bypass subscription)
	isSubscribed := false
	if user.CurrentTeamID != nil {
		isSubscribed = h.service.IsTeamSubscribedOrUserAdmin(*user.CurrentTeamID, userID)
	}

	return response.OK(c, "User retrieved", dto.ToUserResponseWithSubscription(user, isSubscribed))
}

// UpdateProfile updates the user's profile
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateProfileRequest](c)
	if err != nil {
		return err
	}

	user, err := h.service.UpdateProfile(c.Context(), userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Profile updated", dto.ToUserResponse(user))
}

// ChangePassword changes the user's password
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.ChangePasswordRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.ChangePassword(c.Context(), userID, req); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Password changed successfully", nil)
}

// DeleteAccount deletes the user's account
func (h *UserHandler) DeleteAccount(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.DeleteAccount(c.Context(), userID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Account deleted successfully", nil)
}

// CheckUserStatus checks a user's status by email
func (h *UserHandler) CheckUserStatus(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.CheckUserStatusRequest](c)
	if err != nil {
		return err
	}

	status, err := h.service.CheckUserStatus(c.Context(), req.Email)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "User status retrieved", status)
}

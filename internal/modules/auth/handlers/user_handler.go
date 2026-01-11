package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
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
	userID := c.Locals("userID").(string)

	user, err := h.service.GetUser(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	if user == nil {
		return response.NotFound(c, "User not found")
	}

	return response.OK(c, "User retrieved", dto.ToUserResponse(user))
}

// UpdateProfile updates the user's profile
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.UpdateProfileRequest
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

	return response.OK(c, "Profile updated", dto.ToUserResponse(user))
}

// ChangePassword changes the user's password
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.ChangePasswordRequest
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
func (h *UserHandler) DeleteAccount(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	if err := h.service.DeleteAccount(c.Context(), userID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Account deleted successfully", nil)
}

// CheckUserStatus checks a user's status by email
func (h *UserHandler) CheckUserStatus(c *fiber.Ctx) error {
	var req dto.CheckUserStatusRequest
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

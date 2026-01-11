package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// PasswordHandler handles password-related HTTP requests
type PasswordHandler struct {
	service *services.Service
}

// NewPasswordHandler creates a new PasswordHandler instance
func NewPasswordHandler(service *services.Service) *PasswordHandler {
	return &PasswordHandler{service: service}
}

// ForgotPassword initiates password reset
func (h *PasswordHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
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
func (h *PasswordHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
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

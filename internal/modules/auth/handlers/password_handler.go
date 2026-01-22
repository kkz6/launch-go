package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// PasswordHandler handles password-related HTTP requests
type PasswordHandler struct {
	BaseHandler
}

// NewPasswordHandler creates a new PasswordHandler instance
func NewPasswordHandler(service *services.Service) *PasswordHandler {
	return &PasswordHandler{BaseHandler: NewBaseHandler(service)}
}

// ForgotPassword initiates password reset
func (h *PasswordHandler) ForgotPassword(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.ForgotPasswordRequest](c)
	if err != nil {
		return err
	}

	// Always return success to prevent email enumeration
	_ = h.Service().SendPasswordResetLink(c.Context(), req.Email)

	return fiberctx.OK(c, "If an account with that email exists, a password reset link has been sent", nil)
}

// ResetPassword resets the user's password
func (h *PasswordHandler) ResetPassword(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.ResetPasswordRequest](c)
	if err != nil {
		return err
	}

	if err := h.Service().ResetPassword(c.Context(), req); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Password reset successfully", nil)
}

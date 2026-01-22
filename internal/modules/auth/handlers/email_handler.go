package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// EmailHandler handles email verification HTTP requests
type EmailHandler struct {
	BaseHandler
}

// NewEmailHandler creates a new EmailHandler instance
func NewEmailHandler(service *services.Service) *EmailHandler {
	return &EmailHandler{BaseHandler: NewBaseHandler(service)}
}

// VerifyEmail verifies the user's email
func (h *EmailHandler) VerifyEmail(c *fiber.Ctx) error {
	userID := c.Params("id")
	hash := c.Params("hash")

	if err := h.Service().VerifyEmail(c.Context(), userID, hash); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Email verified successfully", nil)
}

// ResendVerificationEmail resends the verification email
func (h *EmailHandler) ResendVerificationEmail(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().ResendVerificationEmail(c.Context(), userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Verification email sent", nil)
}

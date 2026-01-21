package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// EmailHandler handles email verification HTTP requests
type EmailHandler struct {
	service *services.Service
}

// NewEmailHandler creates a new EmailHandler instance
func NewEmailHandler(service *services.Service) *EmailHandler {
	return &EmailHandler{service: service}
}

// VerifyEmail verifies the user's email
func (h *EmailHandler) VerifyEmail(c *fiber.Ctx) error {
	userID := c.Params("id")
	hash := c.Params("hash")

	if err := h.service.VerifyEmail(c.Context(), userID, hash); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Email verified successfully", nil)
}

// ResendVerificationEmail resends the verification email
func (h *EmailHandler) ResendVerificationEmail(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.ResendVerificationEmail(c.Context(), userID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Verification email sent", nil)
}

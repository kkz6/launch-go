package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// TwoFactorHandler handles two-factor authentication HTTP requests
type TwoFactorHandler struct {
	service *services.Service
}

// NewTwoFactorHandler creates a new TwoFactorHandler instance
func NewTwoFactorHandler(service *services.Service) *TwoFactorHandler {
	return &TwoFactorHandler{service: service}
}

// EnableTwoFactor initiates 2FA setup
func (h *TwoFactorHandler) EnableTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	result, err := h.service.EnableTwoFactor(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Two-factor authentication initiated", result)
}

// ConfirmTwoFactor confirms 2FA setup
func (h *TwoFactorHandler) ConfirmTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.ConfirmTwoFactorRequest
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
func (h *TwoFactorHandler) DisableTwoFactor(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.EnableTwoFactorRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.service.DisableTwoFactor(c.Context(), userID, req.Password); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Two-factor authentication disabled", nil)
}

// TwoFactorChallenge verifies 2FA code during login
func (h *TwoFactorHandler) TwoFactorChallenge(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.TwoFactorChallengeRequest
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
func (h *TwoFactorHandler) GetRecoveryCodes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	codes, err := h.service.GetRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Recovery codes retrieved", fiber.Map{"recovery_codes": codes})
}

// RegenerateRecoveryCodes generates new recovery codes
func (h *TwoFactorHandler) RegenerateRecoveryCodes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	codes, err := h.service.RegenerateRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Recovery codes regenerated", fiber.Map{"recovery_codes": codes})
}

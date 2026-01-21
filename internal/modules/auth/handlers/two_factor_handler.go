package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TwoFactorHandler handles two-factor authentication HTTP requests
type TwoFactorHandler struct {
	BaseHandler
}

// NewTwoFactorHandler creates a new TwoFactorHandler instance
func NewTwoFactorHandler(service *services.Service) *TwoFactorHandler {
	return &TwoFactorHandler{BaseHandler: NewBaseHandler(service)}
}

// EnableTwoFactor initiates 2FA setup
func (h *TwoFactorHandler) EnableTwoFactor(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	result, err := h.Service().EnableTwoFactor(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Two-factor authentication initiated", result)
}

// ConfirmTwoFactor confirms 2FA setup
func (h *TwoFactorHandler) ConfirmTwoFactor(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.ConfirmTwoFactorRequest](c)
	if err != nil {
		return err
	}

	if err := h.Service().ConfirmTwoFactor(c.Context(), userID, req.Code); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Two-factor authentication enabled", nil)
}

// DisableTwoFactor disables 2FA
func (h *TwoFactorHandler) DisableTwoFactor(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.EnableTwoFactorRequest](c)
	if err != nil {
		return err
	}

	if err := h.Service().DisableTwoFactor(c.Context(), userID, req.Password); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Two-factor authentication disabled", nil)
}

// TwoFactorChallenge verifies 2FA code during login
func (h *TwoFactorHandler) TwoFactorChallenge(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.TwoFactorChallengeRequest](c)
	if err != nil {
		return err
	}

	code := req.Code
	if code == "" {
		code = req.RecoveryCode
	}

	valid, err := h.Service().VerifyTwoFactor(c.Context(), userID, code)
	if err != nil {
		return response.HandleError(c, err)
	}

	if !valid {
		return response.Unauthorized(c, "Invalid two-factor code")
	}

	return response.OK(c, "Two-factor authentication verified", nil)
}

// GetRecoveryCodes returns the user's recovery codes
func (h *TwoFactorHandler) GetRecoveryCodes(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	codes, err := h.Service().GetRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Recovery codes retrieved", fiber.Map{"recovery_codes": codes})
}

// RegenerateRecoveryCodes generates new recovery codes
func (h *TwoFactorHandler) RegenerateRecoveryCodes(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	codes, err := h.Service().RegenerateRecoveryCodes(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Recovery codes regenerated", fiber.Map{"recovery_codes": codes})
}

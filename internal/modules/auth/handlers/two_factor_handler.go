package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
func (h *TwoFactorHandler) EnableTwoFactor(c *fiber.Ctx, req *dto.EnableTwoFactorRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	result, err := h.Service().TwoFactor.EnableTwoFactor(c.Context(), userID, req.Password)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Two-factor authentication initiated", result)
}

// ConfirmTwoFactor confirms 2FA setup and returns recovery codes
func (h *TwoFactorHandler) ConfirmTwoFactor(c *fiber.Ctx, req *dto.ConfirmTwoFactorRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	codes, err := h.Service().TwoFactor.ConfirmTwoFactor(c.Context(), userID, req.Code)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Two-factor authentication enabled", fiber.Map{"recovery_codes": codes})
}

// DisableTwoFactor disables 2FA
func (h *TwoFactorHandler) DisableTwoFactor(c *fiber.Ctx, req *dto.EnableTwoFactorRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().TwoFactor.DisableTwoFactor(c.Context(), userID, req.Password); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Two-factor authentication disabled", nil)
}

// TwoFactorChallenge verifies 2FA code during login using a challenge token.
// This is a public endpoint — no auth middleware required.
func (h *TwoFactorHandler) TwoFactorChallenge(c *fiber.Ctx, req *dto.TwoFactorChallengeRequest) error {
	code := req.Code
	if code == "" {
		code = req.RecoveryCode
	}

	result, err := h.Service().TwoFactor.CompleteTwoFactorChallenge(c.Context(), req.ChallengeToken, code)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Login successful", result)
}

// GetRecoveryCodes returns the remaining recovery code count.
// Recovery codes are hashed and cannot be retrieved. Use POST to regenerate new codes.
func (h *TwoFactorHandler) GetRecoveryCodes(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	count, err := h.Service().TwoFactor.GetRecoveryCodeCount(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Recovery codes are hashed and cannot be retrieved. Use POST to regenerate.", fiber.Map{"remaining_count": count})
}

// RegenerateRecoveryCodes generates new recovery codes
func (h *TwoFactorHandler) RegenerateRecoveryCodes(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	codes, err := h.Service().TwoFactor.RegenerateRecoveryCodes(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Recovery codes regenerated", fiber.Map{"recovery_codes": codes})
}

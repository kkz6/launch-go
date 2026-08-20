package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func applyAuthResponseLocale(c *fiber.Ctx, response *dto.AuthResponse) {
	if response != nil {
		i18n.ApplyPreference(c, response.User.Locale)
	}
}

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	BaseHandler
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(service *services.Service) *AuthHandler {
	return &AuthHandler{BaseHandler: NewBaseHandler(service)}
}

// Register handles user registration
func (h *AuthHandler) Register(c *fiber.Ctx, req *dto.RegisterRequest) error {
	req.IPAddress = c.IP()
	req.UserAgent = c.Get("User-Agent")

	result, err := h.Service().Auth.Register(c.Context(), req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	applyAuthResponseLocale(c, result)
	return fiberctx.Created(c, "Registration successful", result)
}

// Login handles user authentication.
// If the user has 2FA enabled, returns a challenge token instead of auth tokens.
func (h *AuthHandler) Login(c *fiber.Ctx, req *dto.LoginRequest) error {
	req.IPAddress = c.IP()
	req.UserAgent = c.Get("User-Agent")

	result, err := h.Service().Auth.Login(c.Context(), req)
	if err != nil {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidCredentials)
	}
	i18n.ApplyPreference(c, result.PreferredLocale)

	if result.TwoFactorRequired {
		return fiberctx.OK(c, "Two-factor authentication required", result)
	}

	return fiberctx.OK(c, "Login successful", result)
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	sessionID, _ := c.Locals("sessionID").(string)

	if err := h.Service().Auth.Logout(c.Context(), userID, sessionID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Logged out successfully", nil)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx, req *dto.RefreshTokenRequest) error {
	result, err := h.Service().Auth.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidToken)
	}

	applyAuthResponseLocale(c, result)
	return fiberctx.OK(c, "Token refreshed successfully", result)
}

// TokenExchange exchanges a Personal Access Token for JWT tokens
func (h *AuthHandler) TokenExchange(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidToken)
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidToken)
	}

	result, err := h.Service().Auth.TokenExchange(c.Context(), parts[1])
	if err != nil {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidToken)
	}

	applyAuthResponseLocale(c, result)
	return fiberctx.OK(c, "Token exchanged successfully", result)
}

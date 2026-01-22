package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	BaseHandler
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(service *services.Service) *AuthHandler {
	return &AuthHandler{BaseHandler: NewBaseHandler(service)}
}

// Register handles user registration
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.RegisterRequest](c)
	if err != nil {
		return err
	}

	result, err := h.Service().Register(c.Context(), req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Registration successful", result)
}

// Login handles user authentication
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.LoginRequest](c)
	if err != nil {
		return err
	}

	result, err := h.Service().Login(c.Context(), req)
	if err != nil {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidCredentials)
	}

	return fiberctx.OK(c, "Login successful", result)
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.Service().Logout(c.Context(), userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Logged out successfully", nil)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.RefreshTokenRequest](c)
	if err != nil {
		return err
	}

	result, err := h.Service().RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return fiberctx.RespondUnauthorized(c, fiberctx.MsgInvalidToken)
	}

	return fiberctx.OK(c, "Token refreshed successfully", result)
}

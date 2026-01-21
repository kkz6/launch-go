package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	service *services.Service
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(service *services.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

// Register handles user registration
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.RegisterRequest](c)
	if err != nil {
		return err
	}

	result, err := h.service.Register(c.Context(), req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Registration successful", result)
}

// Login handles user authentication
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.LoginRequest](c)
	if err != nil {
		return err
	}

	result, err := h.service.Login(c.Context(), req)
	if err != nil {
		return response.Unauthorized(c, response.MsgInvalidCredentials)
	}

	return response.OK(c, "Login successful", result)
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.service.Logout(c.Context(), userID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Logged out successfully", nil)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.RefreshTokenRequest](c)
	if err != nil {
		return err
	}

	result, err := h.service.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return response.Unauthorized(c, response.MsgInvalidToken)
	}

	return response.OK(c, "Token refreshed successfully", result)
}

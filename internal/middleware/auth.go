package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// TwoFactorService defines the interface for 2FA operations needed by middlewares
type TwoFactorService interface {
	HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error)
}

// UserService defines the interface for user-related operations needed by middlewares
type UserService interface {
	GetUser(ctx context.Context, userID string) (UserInfo, error)
}

// UserInfo represents minimal user information needed by middlewares
type UserInfo interface {
	HasVerifiedEmail() bool
}

// parseAuthToken extracts and validates the JWT token from the Authorization header.
// Returns the validated claims or an error if the token is missing/invalid.
func parseAuthToken(c *fiber.Ctx, jwtSecret string) (jwt.MapClaims, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, errors.New("invalid authorization header format")
	}

	claims, err := security.ParseJWTToken(parts[1], jwtSecret)
	if err != nil {
		return nil, err
	}

	if err := security.ValidateJWTTokenType(claims, "access"); err != nil {
		return nil, err
	}

	return claims, nil
}

// setAuthContext sets the user context from validated JWT claims.
func setAuthContext(c *fiber.Ctx, claims jwt.MapClaims) {
	userID, _ := security.ExtractJWTClaim(claims, "sub")
	fiberctx.SetUserContext(c, userID, nil)
	c.Locals("email", claims["email"])
}

// Auth middleware requires a valid JWT token in the Authorization header.
// It sets userID and email in request context (c.Locals).
//
// Note: team context is handled by TeamScope middleware which reads
// from X-Team-ID header and validates membership.
func Auth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := parseAuthToken(c, jwtSecret)
		if err != nil {
			return response.Unauthorized(c, "Unauthorized")
		}

		setAuthContext(c, claims)

		return c.Next()
	}
}

// OptionalAuth is like Auth but doesn't require authentication.
// If a valid token is provided, it sets the user context.
// If no token or invalid token, it continues without user context.
//
// Useful for endpoints that work with or without authentication.
func OptionalAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := parseAuthToken(c, jwtSecret)
		if err != nil {
			return c.Next()
		}

		setAuthContext(c, claims)

		return c.Next()
	}
}

// TwoFactor checks if 2FA is required and verified
func TwoFactor(service TwoFactorService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		has2FA, err := service.HasTwoFactorEnabled(c.Context(), userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check 2FA status")
		}

		if !has2FA {
			return c.Next()
		}

		twoFactorVerified := c.Get("X-Two-Factor-Verified")
		if twoFactorVerified == "true" {
			return c.Next()
		}

		return c.Status(fiber.StatusLocked).JSON(fiber.Map{
			"success":             false,
			"message":             "Two-factor authentication required",
			"two_factor_required": true,
		})
	}
}

// EmailVerified checks if the user's email is verified
func EmailVerified(service UserService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "User not authenticated")
		}

		user, err := service.GetUser(c.Context(), userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to get user")
		}

		if user == nil {
			return response.NotFound(c, "User not found")
		}

		if !user.HasVerifiedEmail() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":               false,
				"message":               "Email verification required",
				"email_verified":        false,
				"requires_verification": true,
			})
		}

		return c.Next()
	}
}

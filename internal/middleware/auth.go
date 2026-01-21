package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	jwtutil "github.com/kkz6/launch-go/internal/pkg/jwt"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

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

	claims, err := jwtutil.ParseToken(parts[1], jwtSecret)
	if err != nil {
		return nil, err
	}

	if err := jwtutil.ValidateTokenType(claims, "access"); err != nil {
		return nil, err
	}

	return claims, nil
}

// setAuthContext sets the user context from validated JWT claims.
func setAuthContext(c *fiber.Ctx, claims jwt.MapClaims) {
	userID, _ := jwtutil.ExtractClaim(claims, "sub")
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

package middlewares

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// AuthMiddleware validates JWT tokens and sets user context
func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization header")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}

			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Invalid token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return response.Unauthorized(c, "Invalid token claims")
		}

		// Check token type
		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return response.Unauthorized(c, "Invalid token type")
		}

		// Set user info in context
		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		if teamID, ok := claims["team_id"].(string); ok {
			c.Locals("teamID", teamID)
		}

		return c.Next()
	}
}

// OptionalAuthMiddleware attempts to authenticate but allows anonymous access
func OptionalAuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next() // No auth header, continue as anonymous
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.Next() // Invalid format, continue as anonymous
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}

			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Next() // Invalid token, continue as anonymous
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Next()
		}

		// Check token type
		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return c.Next()
		}

		// Set user info in context
		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		if teamID, ok := claims["team_id"].(string); ok {
			c.Locals("teamID", teamID)
		}

		return c.Next()
	}
}

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

func Auth(jwtSecret string) fiber.Handler {
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

		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return response.Unauthorized(c, "Invalid token type")
		}

		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		// Note: team context is now handled by TeamContext middleware
		// which reads from X-Team-ID header and validates membership

		return c.Next()
	}
}

func OptionalAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.Next()
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Next()
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Next()
		}

		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return c.Next()
		}

		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		// Note: team context is now handled by TeamContext middleware

		return c.Next()
	}
}

package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// UserService defines the interface for user-related operations needed by middlewares
type UserService interface {
	GetUser(ctx context.Context, userID string) (UserInfo, error)
}

// UserInfo represents minimal user information needed by middlewares
type UserInfo interface {
	HasVerifiedEmail() bool
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

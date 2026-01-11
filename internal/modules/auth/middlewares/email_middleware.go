package middlewares

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// EmailVerifiedMiddleware checks if the user's email is verified
func EmailVerifiedMiddleware(service *services.Service) fiber.Handler {
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

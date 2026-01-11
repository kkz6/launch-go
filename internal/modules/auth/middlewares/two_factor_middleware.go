package middlewares

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TwoFactorMiddleware checks if 2FA is required and verified
func TwoFactorMiddleware(service *services.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next() // No user authenticated, let auth middleware handle it
		}

		// Check if user has 2FA enabled
		has2FA, err := service.HasTwoFactorEnabled(c.Context(), userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check 2FA status")
		}

		if !has2FA {
			return c.Next() // 2FA not enabled, proceed
		}

		// Check if 2FA is verified in this session (via custom header or cookie)
		twoFactorVerified := c.Get("X-Two-Factor-Verified")
		if twoFactorVerified == "true" {
			return c.Next()
		}

		// Return 2FA required response
		return c.Status(fiber.StatusLocked).JSON(fiber.Map{
			"success":             false,
			"message":             "Two-factor authentication required",
			"two_factor_required": true,
		})
	}
}

package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TwoFactorService defines the interface for 2FA operations needed by middlewares
type TwoFactorService interface {
	HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error)
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

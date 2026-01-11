package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

func TeamScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Locals("teamID")
		if teamID == nil || teamID == "" {
			return response.Forbidden(c, "Team context required")
		}
		return c.Next()
	}
}

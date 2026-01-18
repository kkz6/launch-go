package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

func getUserIDFromContext(c *fiber.Ctx) *string {
	if userID, ok := c.Locals("userID").(string); ok && userID != "" {
		return &userID
	}

	return nil
}

func getTeamIDFromContext(c *fiber.Ctx) string {
	if teamID, ok := c.Locals("teamID").(string); ok {
		return teamID
	}

	return ""
}

// handleServiceError handles service errors and returns the appropriate HTTP response.
// Since repository and service errors now use response.AppError with HTTP status codes,
// HandleError automatically returns the correct response.
func handleServiceError(c *fiber.Ctx, err error) error {
	return response.HandleError(c, err)
}

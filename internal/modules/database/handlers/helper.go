package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

func getUserIDFromContext(c *fiber.Ctx) *string {
	if userID, ok := c.Locals("userID").(string); ok && userID != "" {
		return &userID
	}

	return nil
}

func handleServiceError(c *fiber.Ctx, err error) error {
	switch err {
	case services.ErrDatabaseNotFound:
		return response.NotFound(c, "Database not found")
	case services.ErrDatabaseUserNotFound:
		return response.NotFound(c, "Database user not found")
	case services.ErrDatabaseNameExists:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case services.ErrDatabaseUserNameExists:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case services.ErrDatabaseBeingUninstalled:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case services.ErrUserBeingUninstalled:
		return response.Error(c, fiber.StatusConflict, err.Error())
	case services.ErrInvalidExistingUser:
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	default:
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}
}

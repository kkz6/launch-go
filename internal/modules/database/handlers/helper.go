package handlers

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func getUserIDFromContext(c *fiber.Ctx) *string {
	userID, err := fiberctx.GetUserID(c)
	if err != nil {
		return nil
	}
	return &userID
}

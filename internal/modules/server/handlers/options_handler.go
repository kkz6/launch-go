package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetCreateOptions returns the static options menu for creating a
// server. Not team-scoped, so it does not fit Index.
func (h *Handler) GetCreateOptions(c *fiber.Ctx) error {
	return fiberctx.OK(c, "Create options retrieved", dto.GetCreateServerOptions())
}

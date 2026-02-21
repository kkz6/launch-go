package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetCreateOptions returns the options for creating a server
func (h *Handler) GetCreateOptions(c *fiber.Ctx) error {
	options := dto.GetCreateServerOptions()
	return fiberctx.OK(c, "Create options retrieved", options)
}

// ListServerProviders returns all connected server providers for the team
func (h *Handler) ListServerProviders(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	providers, err := h.service.ListServerProviders(c.Context(), teamID)
	if err != nil {
		return fiberctx.RespondInternalError(c, "Failed to fetch server providers")
	}

	return fiberctx.OK(c, "Server providers retrieved", pkgdto.TransformSlice(providers, dto.ToServerProviderResponse))
}

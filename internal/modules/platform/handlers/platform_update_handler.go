package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/platform/dto"
	"github.com/kkz6/launch-go/internal/modules/platform/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// PlatformUpdateHandler handles HTTP requests for platform updates
type PlatformUpdateHandler struct {
	service *services.PlatformUpdateService
}

// NewPlatformUpdateHandler creates a new PlatformUpdateHandler
func NewPlatformUpdateHandler(service *services.PlatformUpdateService) *PlatformUpdateHandler {
	return &PlatformUpdateHandler{service: service}
}

// ListPendingUpdates returns updates with pending servers for the team
func (h *PlatformUpdateHandler) ListPendingUpdates(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	updates, err := h.service.GetPendingUpdates(c.Context(), teamID, userID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Platform updates retrieved", updates)
}

// ShowUpdate returns detailed update info with server statuses
func (h *PlatformUpdateHandler) ShowUpdate(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id := c.Params("id")

	detail, err := h.service.GetUpdateDetail(c.Context(), id, teamID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Platform update retrieved", detail)
}

// RunUpdate dispatches the update task on a specific server
func (h *PlatformUpdateHandler) RunUpdate(c *fiber.Ctx, req *dto.RunUpdateRequest) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id := c.Params("id")

	if err := h.service.RunUpdate(c.Context(), id, req.ServerID, teamID); err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Update dispatched", nil)
}

// RunUpdateAll dispatches the update task for all pending servers in the team
func (h *PlatformUpdateHandler) RunUpdateAll(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id := c.Params("id")

	if err := h.service.RunUpdateAll(c.Context(), id, teamID); err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Updates dispatched for all servers", nil)
}

// DismissBanner marks the update banner as dismissed for the current user
func (h *PlatformUpdateHandler) DismissBanner(c *fiber.Ctx) error {
	_, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	id := c.Params("id")

	if err := h.service.DismissBanner(c.Context(), id, userID); err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Update dismissed", nil)
}

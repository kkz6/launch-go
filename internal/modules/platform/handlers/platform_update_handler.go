package handlers

import (
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
func (h *PlatformUpdateHandler) ListPendingUpdates(r *fiberutil.Request) error {
	updates, err := h.service.GetPendingUpdates(r.Context(), r.TeamID, r.UserID)
	if err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Platform updates retrieved", updates)
}

// ShowUpdate returns detailed update info with server statuses
func (h *PlatformUpdateHandler) ShowUpdate(r *fiberutil.Request) error {
	id := r.Params("id")

	detail, err := h.service.GetUpdateDetail(r.Context(), id, r.TeamID)
	if err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Platform update retrieved", detail)
}

// RunUpdate dispatches the update task on a specific server
func (h *PlatformUpdateHandler) RunUpdate(r *fiberutil.Request, req *dto.RunUpdateRequest) error {
	id := r.Params("id")

	if err := h.service.RunUpdate(r.Context(), id, req.ServerID, r.TeamID); err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Update dispatched", nil)
}

// RunUpdateAll dispatches the update task for all pending servers in the team
func (h *PlatformUpdateHandler) RunUpdateAll(r *fiberutil.Request) error {
	id := r.Params("id")

	if err := h.service.RunUpdateAll(r.Context(), id, r.TeamID); err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Updates dispatched for all servers", nil)
}

// DismissBanner marks the update banner as dismissed for the current user
func (h *PlatformUpdateHandler) DismissBanner(r *fiberutil.Request) error {
	id := r.Params("id")

	if err := h.service.DismissBanner(r.Context(), id, r.UserID); err != nil {
		return fiberutil.HandleError(r.Ctx, err)
	}

	return fiberutil.OK(r.Ctx, "Update dismissed", nil)
}

package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// AdminHandler serves the back-office /admin endpoints.
type AdminHandler struct {
	service *services.Service
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(service *services.Service) *AdminHandler {
	return &AdminHandler{service: service}
}

// ListUsers returns a cross-tenant page of users. Proof endpoint for the
// staff gate: only reachable by staff (RequireStaff runs ahead of it).
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	users, total, err := h.service.ListUsers(c.Context(), limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Users retrieved successfully", users, meta)
}

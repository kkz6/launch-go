package handlers

import (
	"github.com/gofiber/fiber/v2"

	staffdto "github.com/kkz6/launch-go/internal/modules/staff/dto"
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

// ListTeams returns a cross-tenant page of teams. Staff-only.
func (h *AdminHandler) ListTeams(c *fiber.Ctx) error {
	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	teams, total, err := h.service.ListTeams(c.Context(), limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Teams retrieved successfully", teams, meta)
}

// ListServers returns a cross-tenant page of servers. Staff-only. Servers are
// mapped to ServerSummary so only an explicit allow-list of safe fields is
// serialized — no secrets ever leave through this endpoint.
func (h *AdminHandler) ListServers(c *fiber.Ctx) error {
	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	servers, total, err := h.service.ListServers(c.Context(), limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	summaries := staffdto.NewServerSummaries(servers)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Servers retrieved successfully", summaries, meta)
}

// ServerLogs returns a server's recent task/activity records. Staff-only.
// Reuses the server module's task store via the injected ServerLogReader —
// this is operational activity, not live log streaming.
func (h *AdminHandler) ServerLogs(c *fiber.Ctx) error {
	serverID := c.Params("id")
	if serverID == "" {
		return fiberutil.RespondBadRequest(c, "Server id is required")
	}

	limit := fiberutil.ParseLimit(c, 50, 200)

	tasks, err := h.service.ServerLogs(c.Context(), serverID, limit)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Server logs retrieved successfully", tasks)
}

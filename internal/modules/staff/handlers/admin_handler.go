package handlers

import (
	"errors"

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

// ListUsers returns a cross-tenant page of users, each row folding in the teams
// the user owns and every team's current subscription status. Staff-only
// (RequireStaff runs ahead of it). Rows are mapped to AdminUserRow so only an
// explicit allow-list of fields is serialized — no passwords or 2FA secrets.
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	rows, total, err := h.service.ListUsersWithBilling(c.Context(), limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Users retrieved successfully", rows, meta)
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

// StartImpersonation begins a read-only "spectate as user" session as the
// target user. The authenticated caller is the staff member (RequireStaff runs
// ahead of this); the impersonator id is taken from the request context, never
// the body. Returns the minted short-lived token plus the audit session.
func (h *AdminHandler) StartImpersonation(c *fiber.Ctx) error {
	staffID, _ := c.Locals(fiberutil.KeyUserID).(string)
	targetUserID := c.Params("userId")

	var body struct {
		Reason string `json:"reason"`
	}
	// reason is optional; ignore body-parse errors
	_ = c.BodyParser(&body)

	token, session, err := h.service.StartImpersonation(c.Context(), staffID, targetUserID, body.Reason)
	if err != nil {
		return mapImpersonationError(c, err)
	}

	return fiberutil.OK(c, "Impersonation started", fiber.Map{
		"token":   token,
		"session": session,
	})
}

// StopImpersonation ends the authenticated staff member's active spectate
// session. The exit flow is driven with the STAFF token, so the session is
// resolved from the authenticated staff identity rather than a request-supplied
// id. A no-op (no active session) still returns 200 so the frontend can safely
// always call stop when leaving spectate mode.
func (h *AdminHandler) StopImpersonation(c *fiber.Ctx) error {
	staffID, _ := c.Locals(fiberutil.KeyUserID).(string)

	stopped, err := h.service.StopImpersonationForStaff(c.Context(), staffID)
	if err != nil {
		return mapImpersonationError(c, err)
	}

	if !stopped {
		return fiberutil.OK(c, "No active impersonation session", nil)
	}

	return fiberutil.OK(c, "Impersonation stopped", nil)
}

// mapImpersonationError translates impersonation service sentinels to sensible
// HTTP statuses. HandleError only maps fiber.Error/ValidationError, so the
// bare sentinels are mapped explicitly here: invalid input -> 400, target not
// found -> 404, and any other error (incl. ErrJWTSecretNotConfigured) falls
// through to HandleError's 500.
func mapImpersonationError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrInvalidImpersonationRequest):
		return fiberutil.RespondBadRequest(c, err.Error())
	case errors.Is(err, services.ErrTargetUserNotFound):
		return fiberutil.RespondNotFound(c, err.Error())
	default:
		return fiberutil.HandleError(c, err)
	}
}

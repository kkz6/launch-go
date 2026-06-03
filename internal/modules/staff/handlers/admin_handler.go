package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
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

// Overview returns the back-office revenue/MRR dashboard computed from existing
// billing data (Stripe-like metrics). Staff-only. The current wall clock is
// passed into the service so month-boundary math stays testable.
func (h *AdminHandler) Overview(c *fiber.Ctx) error {
	overview, err := h.service.Overview(c.Context(), time.Now())
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Overview", overview)
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

// Failures returns a unified, newest-first feed of operational failures across
// provisions, tasks and deployments, read from existing tables. Staff-only. An
// optional `kind` query (provision|task|deployment) narrows the feed to a
// single source; any other value is rejected. Pagination meta carries the total
// across the full merged feed before paging.
func (h *AdminHandler) Failures(c *fiber.Ctx) error {
	kind := c.Query("kind")
	switch kind {
	case "", "provision", "task", "deployment":
		// valid
	default:
		return fiberutil.RespondBadRequest(c, "kind must be one of provision, task, deployment")
	}

	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	response, total, err := h.service.Failures(c.Context(), kind, limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Failures retrieved successfully", response, meta)
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

// SuspendUser freezes a customer account by flipping users.status to
// suspended. Super-admin only (RequireStaff(StaffRoleSuperAdmin) runs ahead of
// it). The actor id is taken from the request context, never the body, so the
// service can enforce the self-suspend guard. Effect is immediate: Auth()
// rejects suspended users on every subsequent request.
func (h *AdminHandler) SuspendUser(c *fiber.Ctx) error {
	targetID := c.Params("id")
	if targetID == "" {
		return fiberutil.RespondBadRequest(c, "User id is required")
	}

	actorID, _ := c.Locals(fiberutil.KeyUserID).(string)

	if err := h.service.SetUserStatus(c.Context(), actorID, targetID, authtypes.UserStatusSuspended); err != nil {
		return mapUserStatusError(c, err)
	}

	return fiberutil.OK(c, "User suspended", fiber.Map{
		"id":     targetID,
		"status": authtypes.UserStatusSuspended.String(),
	})
}

// UnsuspendUser restores a suspended customer account to active. Super-admin
// only. Same context/guard handling as SuspendUser.
func (h *AdminHandler) UnsuspendUser(c *fiber.Ctx) error {
	targetID := c.Params("id")
	if targetID == "" {
		return fiberutil.RespondBadRequest(c, "User id is required")
	}

	actorID, _ := c.Locals(fiberutil.KeyUserID).(string)

	if err := h.service.SetUserStatus(c.Context(), actorID, targetID, authtypes.UserStatusActive); err != nil {
		return mapUserStatusError(c, err)
	}

	return fiberutil.OK(c, "User unsuspended", fiber.Map{
		"id":     targetID,
		"status": authtypes.UserStatusActive.String(),
	})
}

// DeleteUser hard-deletes a customer account. Super-admin only
// (RequireStaff(StaffRoleSuperAdmin) runs ahead of it). This is a DESTRUCTIVE
// operation: deleting the users row cascades at the DB level to every resource
// the user owns (teams, servers, sites, ssh keys, providers, ...). It is only
// permitted when the user never had a paying/active subscription and never
// placed a paid order, and never for a staff member or the actor themselves.
//
// The actor id is taken from the request context, never the body, so the
// service can enforce the self-delete guard. The deletability check runs first
// to surface a precise 409 reason; the service then re-checks the guards inside
// its delete transaction so a lost race can never delete a paying user.
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	targetID := c.Params("id")
	if targetID == "" {
		return fiberutil.RespondBadRequest(c, "User id is required")
	}

	actorID, _ := c.Locals(fiberutil.KeyUserID).(string)

	if actorID != "" && actorID == targetID {
		return fiberutil.RespondConflict(c, services.ErrCannotDeleteSelf.Error())
	}

	deletable, reason, err := h.service.CanDeleteUser(c.Context(), targetID)
	if err != nil {
		return mapUserDeleteError(c, err)
	}

	if !deletable {
		return fiberutil.RespondConflict(c, reason)
	}

	if err := h.service.DeleteUser(c.Context(), actorID, targetID); err != nil {
		return mapUserDeleteError(c, err)
	}

	return fiberutil.OK(c, "User deleted", fiber.Map{"id": targetID})
}

// CreateInvitation issues a platform-level invitation with a trial period and
// emails the recipient an invite link. Super-admin only (RequireStaff(
// StaffRoleSuperAdmin) runs ahead of it). The inviter id is taken from the
// request context, never the body. The body carries the target email and the
// trial end date (RFC3339, or a YYYY-MM-DD date). The invitation JSON omits the
// token (json:"-"), so the secret never leaves through the response.
func (h *AdminHandler) CreateInvitation(c *fiber.Ctx) error {
	var body struct {
		Email       string `json:"email"`
		TrialEndsAt string `json:"trial_ends_at"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}

	if body.Email == "" {
		return fiberutil.RespondBadRequest(c, "Email is required")
	}

	trialEndsAt, err := parseTrialEndsAt(body.TrialEndsAt)
	if err != nil {
		return fiberutil.RespondBadRequest(c, "trial_ends_at must be an RFC3339 timestamp or a YYYY-MM-DD date")
	}

	actorID, _ := c.Locals(fiberutil.KeyUserID).(string)

	invitation, err := h.service.InviteUser(c.Context(), body.Email, trialEndsAt, actorID)
	if err != nil {
		return mapInvitationError(c, err)
	}

	return fiberutil.OK(c, "Invitation sent", invitation)
}

// ListInvitations returns a page of pending (non-accepted, non-expired)
// platform invitations. Support-tier (the group gate already applies).
func (h *AdminHandler) ListInvitations(c *fiber.Ctx) error {
	limit := fiberutil.ParseLimit(c, 25, 100)
	offset := fiberutil.ParseOffset(c)

	invitations, total, err := h.service.ListPendingInvitations(c.Context(), limit, offset)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	page := offset/limit + 1
	meta := dto.NewPaginationMeta(page, limit, total)

	return fiberutil.SuccessWithMeta(c, fiber.StatusOK, "Invitations retrieved successfully", invitations, meta)
}

// RevokeInvitation deletes a pending invitation by id. Super-admin only. A
// missing invitation yields a 404.
func (h *AdminHandler) RevokeInvitation(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiberutil.RespondBadRequest(c, "Invitation id is required")
	}

	if err := h.service.RevokeInvitation(c.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.RespondNotFound(c, "Invitation not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Invitation revoked", fiber.Map{"id": id})
}

// parseTrialEndsAt accepts either an RFC3339 timestamp or a bare YYYY-MM-DD
// date for the trial end. A bare date is interpreted at UTC midnight.
func parseTrialEndsAt(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("trial_ends_at is required")
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}

	return time.Parse("2006-01-02", raw)
}

// mapInvitationError translates invitation service sentinels to HTTP statuses:
// an existing user or a pending invite -> 409; an invalid email or trial date ->
// 400. An email-delivery failure wraps the created invitation and is not a
// sentinel, so it falls through to HandleError's 500 — the invite row still
// exists and the link is usable; the error just tells the admin the mail did
// not go out.
func mapInvitationError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrUserAlreadyExists):
		return fiberutil.RespondConflict(c, err.Error())
	case errors.Is(err, services.ErrInvitePending):
		return fiberutil.RespondConflict(c, err.Error())
	case errors.Is(err, services.ErrInvalidTrialDate):
		return fiberutil.RespondBadRequest(c, err.Error())
	case errors.Is(err, services.ErrInvalidInviteEmail):
		return fiberutil.RespondBadRequest(c, err.Error())
	default:
		return fiberutil.HandleError(c, err)
	}
}

// mapUserDeleteError translates delete service sentinels to HTTP statuses:
// missing target -> 404; self-delete or a not-deletable user (staff role, paid
// order, paid subscription — including a lost-race refusal from the in-tx
// re-check) -> 409 carrying the reason; anything else falls through to
// HandleError's 500.
func mapUserDeleteError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		return fiberutil.RespondNotFound(c, err.Error())
	case errors.Is(err, services.ErrCannotDeleteSelf):
		return fiberutil.RespondConflict(c, err.Error())
	}

	var notDeletable *services.NotDeletableError
	if errors.As(err, &notDeletable) {
		return fiberutil.RespondConflict(c, notDeletable.Reason)
	}

	return fiberutil.HandleError(c, err)
}

// mapUserStatusError translates suspend/unsuspend service sentinels to HTTP
// statuses: missing target -> 404; suspending a staff member or yourself -> 409
// (a conflict with the resource's protected state); anything else falls through
// to HandleError's 500.
func mapUserStatusError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		return fiberutil.RespondNotFound(c, err.Error())
	case errors.Is(err, services.ErrCannotSuspendStaff):
		return fiberutil.RespondConflict(c, err.Error())
	case errors.Is(err, services.ErrCannotSuspendSelf):
		return fiberutil.RespondConflict(c, err.Error())
	default:
		return fiberutil.HandleError(c, err)
	}
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

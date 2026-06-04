package staff

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/staff/handlers"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/modules/staff/tables"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// ModuleName is the staff module's unique identifier.
const ModuleName = "staff"

// Ensure Module implements required interfaces.
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module is the staff back-office module. Staff access is a product-wide axis,
// independent of any customer team.
type Module struct {
	app.Base
	db      *gorm.DB
	repos   *repositories.Registry
	service *services.Service
	handler *handlers.AdminHandler
}

// NewModule creates a new staff module.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	service := services.NewService(repos)

	// Wire the JWT signing secret used to mint scoped impersonation tokens.
	// Sourced from config (never the environment directly) at construction time,
	// since the builder exposes config here.
	if deps.Config != nil {
		service.SetJWTSecret(deps.Config.JWT.Secret)
	}

	handler := handlers.NewAdminHandler(service)

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		db:      deps.DB,
		repos:   repos,
		service: service,
		handler: handler,
	}
}

// Service returns the staff service. Used by bootstrap to wire the
// RequireStaff middleware loader.
func (m *Module) Service() *services.Service {
	return m.service
}

// SetServerLogReader wires the cross-module reader used by the server-logs
// endpoint to fetch a server's recent task/activity records. Injected from
// main.go with the server module's task repository.
func (m *Module) SetServerLogReader(reader services.ServerLogReader) {
	m.service.SetServerLogReader(reader)
}

// RegisterRoutes registers the back-office /admin routes behind the staff gate.
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	admin := router.Group("/admin", middleware.StaffChain(authMiddleware, stafftypes.StaffRoleSupport)...)
	admin.Get("/overview", m.handler.Overview)
	admin.Get("/users", m.handler.ListUsers)
	admin.Get("/users/:id", m.handler.ShowUser)
	admin.Get("/teams", m.handler.ListTeams)
	admin.Get("/servers", m.handler.ListServers)
	admin.Get("/servers/:id", m.handler.ShowServer)
	admin.Get("/servers/:id/logs", m.handler.ServerLogs)
	admin.Get("/failures", m.handler.Failures)

	// Impersonation is driven with the STAFF token: start mints a scoped
	// read-only token for the target; stop ends the staff member's active
	// session. Both run under StaffChain, so an impersonation token (sub=target)
	// can't reach them — you must hold the staff token to start or stop.
	// The static /stop route is registered before the /:userId param route so
	// Fiber matches it first.
	admin.Post("/impersonate/stop", m.handler.StopImpersonation)
	admin.Post("/impersonate/:userId", m.handler.StartImpersonation)

	// Suspend/unsuspend freeze or restore a customer account. These are
	// destructive account-state writes, so they require super_admin on top of
	// the support-tier gate the group already applies: the chain runs auth ->
	// RequireStaff(support) (from StaffChain) -> RequireStaff(super_admin) ->
	// handler. A support-tier staffer clears the group gate but is rejected by
	// the inline super_admin check.
	admin.Post("/users/:id/suspend", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), m.handler.SuspendUser)
	admin.Post("/users/:id/unsuspend", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), m.handler.UnsuspendUser)

	// Delete is the single DESTRUCTIVE account action: it removes the users row,
	// which cascades at the DB level to every resource the user owns. It is
	// gated identically to suspend (super_admin on top of the support-tier
	// group gate) and additionally refuses any user who ever paid (paid order
	// or non-trial subscription), holds a staff role, or is the actor.
	admin.Delete("/users/:id", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), m.handler.DeleteUser)

	// Platform invitations: super_admin creates and revokes (destructive /
	// account-creating writes), support-tier may read the pending list. Create
	// emails the recipient a trial invite link; revoke deletes a pending invite.
	admin.Post("/invitations", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), m.handler.CreateInvitation)
	admin.Get("/invitations", m.handler.ListInvitations)
	admin.Delete("/invitations/:id", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), m.handler.RevokeInvitation)

	// Declarative invitations table. Reads (/meta, /data) inherit the group's
	// support-tier gate; the mutating /action/:name route is additionally gated
	// behind super_admin via the action middleware, matching the REST revoke
	// endpoint above. The REST endpoints stay in place — this table is additive
	// while the frontend migrates.
	tableQuery := table.NewQueryService(m.db)
	tableViews := table.NewViewService(m.db)
	invTable := tables.NewInvitationsTable(m.service)
	invGroup := admin.Group("/invitations/table")
	table.Mount(invGroup, invTable, tableQuery, tableViews, middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin))

	// Declarative servers table. Read-only: only safe columns are declared, so
	// the declared-column projection guarantees no secret server column (SSH
	// keys, tokens, passwords) reaches /data. No row actions, so no action
	// middleware is needed. The REST /admin/servers endpoint above stays in
	// place — this table is additive.
	serversTable := tables.NewServersTable()
	table.Mount(admin.Group("/servers/table"), serversTable, tableQuery, tableViews)

	// Declarative failures table. A pure Resolver table: it owns its data fetch
	// by delegating to the Failures service (merging provision/task/deployment
	// failures), so the model-driven query path is skipped. Read-only — no row
	// actions, so no action middleware is mounted. The REST /admin/failures
	// endpoint above stays in place — this table is additive.
	failuresTable := tables.NewFailuresTable(m.service)
	table.Mount(admin.Group("/failures/table"), failuresTable, tableQuery, tableViews)

	// Declarative users table. A pure Resolver table: it owns its data fetch by
	// delegating to ListUsersWithBilling (user + owned teams + each team's
	// subscription status). Reads (/meta, /data) inherit the group's support-tier
	// gate; the mutating /action/:name route (suspend, unsuspend, delete) is
	// additionally gated behind super_admin via the action middleware, matching
	// the REST suspend/unsuspend/delete endpoints above. The REST /admin/users
	// endpoint stays in place — this table is additive.
	usersTable := tables.NewUsersTable(m.service)
	table.Mount(admin.Group("/users/table"), usersTable, tableQuery, tableViews, middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin))
}

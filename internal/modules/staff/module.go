package staff

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/staff/handlers"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/app"
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
	admin.Get("/teams", m.handler.ListTeams)
	admin.Get("/servers", m.handler.ListServers)
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
}

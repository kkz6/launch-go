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
	admin.Get("/users", m.handler.ListUsers)
	admin.Get("/teams", m.handler.ListTeams)
	admin.Get("/servers", m.handler.ListServers)
	admin.Get("/servers/:id/logs", m.handler.ServerLogs)
}

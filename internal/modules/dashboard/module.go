package dashboard

import (
	"github.com/kkz6/launch-go/internal/modules/dashboard/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "dashboard"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the dashboard module
type Module struct {
	app.Base
	service *services.DashboardService
}

// NewModule creates a new dashboard module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()

	service := services.NewDashboardService(deps.DB, deps.Logger)

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		service: service,
	}
}

package dashboard

import (
	"github.com/kkz6/launch-go/internal/modules/dashboard/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "dashboard"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the dashboard module
type Module struct {
	module.Base
	service *services.DashboardService
}

// NewModule creates a new dashboard module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	service := services.NewDashboardService(deps.DB, deps.Logger)

	return &Module{
		Base:    module.NewBase(ModuleName, b),
		service: service,
	}
}

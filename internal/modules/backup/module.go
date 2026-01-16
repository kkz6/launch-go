package backup

import (
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "backup"

// Ensure Module implements required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.RouteRegistrar   = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
)

// Module represents the backup module
type Module struct {
	module.Base

	// Repository registry
	repos *repositories.Registry
}

// NewModule creates a new backup module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:  module.NewBase(ModuleName, b),
		repos: repositories.NewRegistry(deps.DB),
	}
}

// createServices creates all services needed for route handlers
func (m *Module) createServices() *services.ServiceRegistry {
	deps := m.Deps()

	// Create shared service dependencies
	svcDeps := &services.ServiceDeps{
		DB:        deps.DB,
		Logger:    deps.Logger,
		Queue:     deps.Queue,
		WebSocket: deps.WebSocket,
		Repos:     m.repos,
	}

	// Create service registry - handles all service creation and wiring
	return services.NewServiceRegistry(svcDeps)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

package dockerregistry

import (
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/repositories"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// ModuleName names this module.
const ModuleName = "dockerregistry"

// Ensure Module implements required interfaces.
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module wires docker-registry repositories, services, and routes into
// the application kernel.
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.Service
}

// NewModule constructs the docker-registry module.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)

	service := services.NewService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: service,
	}
}

// Repos returns the repository registry.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// Service returns the service for cross-module wiring.
func (m *Module) Service() *services.Service { return m.service }

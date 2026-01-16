package auth

import (
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "auth"

// Ensure Module implements required interfaces
var (
	_ app.Module               = (*Module)(nil)
	_ app.PublicRouteRegistrar = (*Module)(nil)
)

// Module represents the auth module with all its dependencies
type Module struct {
	module.Base
	service     *services.Service
	repository  *repositories.Repository
	passkeyRepo *repositories.PasskeyRepository
}

// NewModule creates a new auth Module instance
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	repo := repositories.NewRepository(deps.DB)
	passkeyRepo := repositories.NewPasskeyRepository(deps.DB)
	service := services.NewService(repo, deps.Config, deps.Logger)

	return &Module{
		Base:        module.NewBase(ModuleName, b),
		service:     service,
		repository:  repo,
		passkeyRepo: passkeyRepo,
	}
}

// Service returns the auth service
func (m *Module) Service() *services.Service {
	return m.service
}

// Repository returns the auth repository
func (m *Module) Repository() *repositories.Repository {
	return m.repository
}

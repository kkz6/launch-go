package auth

import (
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/cache"
)

const ModuleName = "auth"

// Ensure Module implements required interfaces
var (
	_ app.Module               = (*Module)(nil)
	_ app.PublicRouteRegistrar = (*Module)(nil)
)

// Module represents the auth module with all its dependencies
type Module struct {
	app.Base
	service *services.Service
	repos   *repositories.Registry
	cache   cache.Cache
}

// NewModule creates a new auth Module instance
func NewModule(b *app.Builder, emailSender channels.EmailSender, redisCache cache.Cache) (*Module, error) {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)

	service, err := services.NewService(repos, deps.Config, deps.Logger, emailSender, redisCache)
	if err != nil {
		return nil, err
	}

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		service: service,
		repos:   repos,
		cache:   redisCache,
	}, nil
}

// Service returns the auth service
func (m *Module) Service() *services.Service {
	return m.service
}

// Repos returns the auth repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

package auth

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	"github.com/kkz6/launch-go/internal/modules/auth/policies"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/pkg/access"
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
	gate    *access.Gate
}

// NewModule creates a new auth Module instance
func NewModule(b *app.Builder, emailSender channels.EmailSender, redisCache cache.Cache) (*Module, error) {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)

	service, err := services.NewService(repos, deps.Config, deps.Logger, emailSender, redisCache)
	if err != nil {
		return nil, err
	}

	// Wire the shared team-membership cache so /auth/user can surface the
	// caller's role in their current team (drives frontend UI gating).
	service.SetMembershipCache(deps.MembershipCache)

	// Build the shared authorization gate. The auth module owns it; other
	// modules register their policies into it at boot via authModule.Gate().
	gate := access.New()
	gate.Before(authaccess.ReadOnlyFreeze) // deny mutations under read-only impersonation
	policies.Register(gate)                // auth's own team/member abilities

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		service: service,
		repos:   repos,
		cache:   redisCache,
		gate:    gate,
	}, nil
}

// Gate returns the shared authorization gate. Other modules register their
// policies into it and the Can middleware authorizes against it.
func (m *Module) Gate() *access.Gate {
	return m.gate
}

// Service returns the auth service
func (m *Module) Service() *services.Service {
	return m.service
}

// Repos returns the auth repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

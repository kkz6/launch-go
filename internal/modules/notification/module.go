package notification

import (
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "notification"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the notification module
type Module struct {
	module.Base

	// Repository registry
	repos *repositories.Registry

	// Channel factory (needed for creating notification channels)
	channelFactory *channels.Factory
}

// NewModule creates a new notification module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	httpClient := channels.NewDefaultHTTPClient()
	channelFactory := channels.NewFactory(httpClient)

	return &Module{
		Base:           module.NewBase(ModuleName, b),
		repos:          repositories.NewRegistry(deps.DB),
		channelFactory: channelFactory,
	}
}

// createServices creates all services needed for route handlers
func (m *Module) createServices() *services.ServiceRegistry {
	deps := m.Deps()

	// Get admin webhook URL from config
	adminWebhookURL := ""
	if deps.Config != nil {
		adminWebhookURL = deps.Config.Slack.AdminWebhookURL
	}

	// Create shared service dependencies using embedded service.ModuleDeps
	svcDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ChannelFactory:  m.channelFactory,
		AdminWebhookURL: adminWebhookURL,
	}

	// Create service registry - handles all service creation and wiring
	return services.NewServiceRegistry(svcDeps)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

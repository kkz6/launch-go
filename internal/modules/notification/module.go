package notification

import (
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
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
	app.Base

	// Repository registry
	repos *repositories.Registry

	// Channel factory (needed for creating notification channels)
	channelFactory *channels.Factory

	// Cached service registry (lazily created by createServices)
	serviceRegistry *services.ServiceRegistry
}

// NewModule creates a new notification module
func NewModule(b *app.Builder, emailSender channels.EmailSender) *Module {
	deps := b.Deps()
	httpClient := channels.NewDefaultHTTPClient()

	var channelFactory *channels.Factory
	if emailSender != nil {
		channelFactory = channels.NewFactoryWithEmail(httpClient, emailSender)
	} else {
		channelFactory = channels.NewFactory(httpClient)
	}

	return &Module{
		Base:           app.NewBase(ModuleName, b),
		repos:          repositories.NewRegistry(deps.DB),
		channelFactory: channelFactory,
	}
}

// createServices creates all services needed for route handlers.
// The result is cached so subsequent calls return the same registry.
func (m *Module) createServices() *services.ServiceRegistry {
	if m.serviceRegistry != nil {
		return m.serviceRegistry
	}

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
	m.serviceRegistry = services.NewServiceRegistry(svcDeps)
	return m.serviceRegistry
}

// Notifier returns a NotifierAdapter that satisfies the taskrunner.NotifierService
// interface. It lazily creates the service registry if needed.
func (m *Module) Notifier() *NotifierAdapter {
	svc := m.createServices()
	return NewNotifierAdapter(svc.Notifier())
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

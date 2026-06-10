package billing

import (
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "billing"

// Ensure Module implements required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.RouteRegistrar   = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
)

// Module represents the billing module
type Module struct {
	app.Base
	service        *services.BillingService
	webhookService *services.WebhookService
	repos          *repositories.Registry
	polarClient    *providers.PolarClient
	serverCountFn  func(teamID string) (int, error)
}

// NewModule creates a new billing module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	cfg := deps.Config.Billing

	repos := repositories.NewRegistry(deps.DB)

	var polarClient *providers.PolarClient
	if cfg.Polar.AccessToken != "" {
		polarClient = providers.NewPolarClient(&providers.PolarConfig{
			AccessToken:    cfg.Polar.AccessToken,
			WebhookSecret:  cfg.Polar.WebhookSecret,
			OrganizationID: cfg.Polar.OrganizationID,
			Sandbox:        cfg.Polar.Sandbox,
		}, deps.Logger)
	}

	billingConfig := &services.Config{
		SubscriptionsEnabled: cfg.SubscriptionsEnabled,
		Plans:                models.PlansFromConfig(cfg.Polar.ProductHobby, cfg.Polar.ProductCompact, cfg.Polar.ProductTurbo),
	}

	// NewBillingService takes a BillingProvider; pass nil explicitly when Polar
	// isn't configured so the interface value is a true nil (not a typed nil).
	var provider providers.BillingProvider
	if polarClient != nil {
		provider = polarClient
	}

	service := services.NewBillingService(repos, provider, billingConfig, deps.Logger)
	webhookService := services.NewWebhookService(repos)

	return &Module{
		Base:           app.NewBase(ModuleName, b),
		service:        service,
		webhookService: webhookService,
		repos:          repos,
		polarClient:    polarClient,
	}
}

// SetServerCountFn sets the function to count servers for a team
func (m *Module) SetServerCountFn(fn func(teamID string) (int, error)) {
	m.serverCountFn = fn
}

// GetService returns the billing service
func (m *Module) GetService() *services.BillingService {
	return m.service
}

// Repos returns the billing repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

// DefaultPlans returns default plan configurations
func DefaultPlans() []models.Plan {
	return models.DefaultPlans()
}

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
	serverCountFn  func(teamID string) (int, error)
}

// NewModule creates a new billing module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	cfg := deps.Config.Billing

	repos := repositories.NewRegistry(deps.DB)

	var lsClient *providers.LemonSqueezyClient
	if cfg.LemonSqueezy.APIKey != "" {
		lsConfig := &providers.LemonSqueezyConfig{
			APIKey:  cfg.LemonSqueezy.APIKey,
			StoreID: cfg.LemonSqueezy.StoreID,
		}
		lsClient = providers.NewLemonSqueezyClient(lsConfig, deps.Logger)
	}

	billingConfig := &services.Config{
		SubscriptionsEnabled: cfg.SubscriptionsEnabled,
		Plans:                models.DefaultPlans(),
	}

	service := services.NewBillingService(repos, lsClient, billingConfig, deps.Logger)
	webhookService := services.NewWebhookService(repos)

	return &Module{
		Base:           app.NewBase(ModuleName, b),
		service:        service,
		webhookService: webhookService,
		repos:          repos,
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

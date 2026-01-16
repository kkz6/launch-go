package billing

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
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
	module.Base
	service       *services.BillingService
	repos         *repositories.Registry
	serverCountFn func(teamID string) (int, error)
}

// NewModule creates a new billing module
func NewModule(b *module.Builder) *Module {
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

	return &Module{
		Base:    module.NewBase(ModuleName, b),
		service: service,
		repos:   repos,
	}
}

// SetServerCountFn sets the function to count servers for a team
func (m *Module) SetServerCountFn(fn func(teamID string) (int, error)) {
	m.serverCountFn = fn
}

// RegisterRoutes registers the module routes (implements app.RouteRegistrar)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewBillingHandler(m.service, m.serverCountFn)

	billing := router.Group("/billing", authMiddleware)

	billing.Get("/", handler.Index)
	billing.Get("/plans", handler.GetPlans)
	billing.Post("/checkout-url", handler.GenerateCheckoutURL)
	billing.Post("/cancel-subscription", handler.CancelSubscription)
	billing.Post("/resume-subscription", handler.ResumeSubscription)

	billing.Get("/subscriptions", handler.GetSubscriptions)
	billing.Get("/subscriptions/:id", handler.GetSubscription)
	billing.Get("/orders", handler.GetOrders)
	billing.Get("/options", handler.GetSubscriptionOptions)

	router.Get("/register/subscription", authMiddleware, handler.RegisterSubscription)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()
	webhookHandler := handlers.NewWebhookHandler(m.repos, m.service, deps.Config.Billing.WebhookSecret, deps.Logger)
	router.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)
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

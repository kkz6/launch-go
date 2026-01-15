package billing

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// Ensure Module implements required interfaces
var (
	_ app.Module            = (*Module)(nil)
	_ app.RouteRegistrar    = (*Module)(nil)
	_ app.WebhookRegistrar  = (*Module)(nil)
)

// Module represents the billing module
type Module struct {
	handler        *handlers.BillingHandler
	webhookHandler *handlers.WebhookHandler
	service        *services.BillingService
	repo           *repositories.BillingRepository
	config         *ModuleConfig
}

// ModuleConfig holds configuration for the billing module
type ModuleConfig struct {
	DB                   *gorm.DB
	Logger               *zerolog.Logger
	SubscriptionsEnabled bool
	Plans                []models.Plan
	LemonSqueezy         *providers.LemonSqueezyConfig
	WebhookSecret        string
	ServerCountFn        func(teamID string) (int, error)
}

// NewModule creates a new billing module
func NewModule(config *ModuleConfig) *Module {
	repo := repositories.NewBillingRepository(config.DB)

	var lsClient *providers.LemonSqueezyClient
	if config.LemonSqueezy != nil && config.LemonSqueezy.APIKey != "" {
		lsClient = providers.NewLemonSqueezyClient(config.LemonSqueezy, config.Logger)
	}

	billingConfig := &services.Config{
		SubscriptionsEnabled: config.SubscriptionsEnabled,
		Plans:                config.Plans,
	}

	service := services.NewBillingService(repo, lsClient, billingConfig, config.Logger)
	handler := handlers.NewBillingHandler(service, config.ServerCountFn)
	webhookHandler := handlers.NewWebhookHandler(repo, service, config.WebhookSecret, config.Logger)

	return &Module{
		handler:        handler,
		webhookHandler: webhookHandler,
		service:        service,
		repo:           repo,
		config:         config,
	}
}

// NewModuleFromContext creates a billing module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	config := &ModuleConfig{
		DB:                   ctx.DB,
		Logger:               ctx.Logger,
		SubscriptionsEnabled: ctx.Config.Billing.SubscriptionsEnabled,
		Plans:                DefaultPlans(),
		WebhookSecret:        ctx.Config.Billing.WebhookSecret,
	}

	if ctx.Config.Billing.LemonSqueezy.APIKey != "" {
		config.LemonSqueezy = &providers.LemonSqueezyConfig{
			APIKey:  ctx.Config.Billing.LemonSqueezy.APIKey,
			StoreID: ctx.Config.Billing.LemonSqueezy.StoreID,
		}
	}

	return NewModule(config)
}

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "billing"
}

// RegisterRoutes registers the module routes (implements app.RouteRegistrar)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	billing := router.Group("/billing", authMiddleware)

	billing.Get("/", m.handler.Index)
	billing.Get("/plans", m.handler.GetPlans)
	billing.Post("/checkout-url", m.handler.GenerateCheckoutURL)
	billing.Post("/cancel-subscription", m.handler.CancelSubscription)
	billing.Post("/resume-subscription", m.handler.ResumeSubscription)

	billing.Get("/subscriptions", m.handler.GetSubscriptions)
	billing.Get("/subscriptions/:id", m.handler.GetSubscription)
	billing.Get("/orders", m.handler.GetOrders)
	billing.Get("/options", m.handler.GetSubscriptionOptions)

	router.Get("/register/subscription", authMiddleware, m.handler.RegisterSubscription)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	router.Post("/webhooks/lemon-squeezy", m.webhookHandler.HandleWebhook)
}

// GetService returns the billing service
func (m *Module) GetService() *services.BillingService {
	return m.service
}

// GetRepository returns the billing repository
func (m *Module) GetRepository() *repositories.BillingRepository {
	return m.repo
}

// GetWebhookHandler returns the webhook handler
func (m *Module) GetWebhookHandler() *handlers.WebhookHandler {
	return m.webhookHandler
}

// Migrate runs database migrations for the billing module
func (m *Module) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Subscription{},
		&models.Order{},
		&models.WebhookEvent{},
	)
}

// DefaultPlans returns default plan configurations
func DefaultPlans() []models.Plan {
	return models.DefaultPlans()
}

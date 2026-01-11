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
)

// Module represents the billing module
type Module struct {
	handler        *handlers.BillingHandler
	webhookHandler *handlers.WebhookHandler
	service        *services.BillingService
	repo           *repositories.BillingRepository
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
	}
}

// RegisterRoutes registers the module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handlers.RegisterRoutes(router, m.handler, m.webhookHandler, authMiddleware)
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

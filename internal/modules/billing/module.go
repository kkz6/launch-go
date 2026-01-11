package billing

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Module represents the billing module
type Module struct {
	handler        *Handler
	webhookHandler *WebhookHandler
	service        *Service
	repo           *Repository
}

// ModuleConfig holds configuration for the billing module
type ModuleConfig struct {
	DB                   *gorm.DB
	Logger               *zerolog.Logger
	SubscriptionsEnabled bool
	Plans                []Plan
	LemonSqueezy         *LemonSqueezyConfig
	WebhookSecret        string
	ServerCountFn        func(teamID string) (int, error)
}

// NewModule creates a new billing module
func NewModule(config *ModuleConfig) *Module {
	repo := NewRepository(config.DB)

	var lsClient *LemonSqueezyClient
	if config.LemonSqueezy != nil && config.LemonSqueezy.APIKey != "" {
		lsClient = NewLemonSqueezyClient(config.LemonSqueezy, config.Logger)
	}

	billingConfig := &Config{
		SubscriptionsEnabled: config.SubscriptionsEnabled,
		Plans:                config.Plans,
	}

	service := NewService(repo, lsClient, billingConfig, config.Logger)
	handler := NewHandler(service, config.ServerCountFn)
	webhookHandler := NewWebhookHandler(repo, service, config.WebhookSecret, config.Logger)

	return &Module{
		handler:        handler,
		webhookHandler: webhookHandler,
		service:        service,
		repo:           repo,
	}
}

// RegisterRoutes registers the module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	RegisterRoutes(router, m.handler, m.webhookHandler, authMiddleware)
}

// GetService returns the billing service
func (m *Module) GetService() *Service {
	return m.service
}

// GetRepository returns the billing repository
func (m *Module) GetRepository() *Repository {
	return m.repo
}

// GetWebhookHandler returns the webhook handler
func (m *Module) GetWebhookHandler() *WebhookHandler {
	return m.webhookHandler
}

// Migrate runs database migrations for the billing module
func (m *Module) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Subscription{},
		&Order{},
		&WebhookEvent{},
	)
}

// DefaultPlans returns default plan configurations
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:             "1",
			Name:           "Starter",
			Description:    "Perfect for small projects",
			MonthlyID:      "starter_monthly",
			YearlyID:       "starter_yearly",
			MonthlyPricing: 900,
			YearlyPricing:  9000,
			Features: []string{
				"Up to 3 servers",
				"Up to 5 sites per server",
				"5 deployments retained",
				"2 team members",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            3,
				MaxSitesPerServer:     5,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        2,
				HasBackups:            false,
				HasMonitoring:         false,
			},
		},
		{
			ID:             "2",
			Name:           "Pro",
			Description:    "For growing teams",
			MonthlyID:      "pro_monthly",
			YearlyID:       "pro_yearly",
			MonthlyPricing: 2900,
			YearlyPricing:  29000,
			Features: []string{
				"Up to 10 servers",
				"Up to 20 sites per server",
				"10 deployments retained",
				"5 team members",
				"Server monitoring",
			},
			Recommended: true,
			Options: PlanOptions{
				MaxServers:            10,
				MaxSitesPerServer:     20,
				MaxDeploymentsPerSite: 10,
				MaxTeamMembers:        5,
				HasBackups:            false,
				HasMonitoring:         true,
			},
		},
		{
			ID:             "3",
			Name:           "Enterprise",
			Description:    "For large organizations",
			MonthlyID:      "enterprise_monthly",
			YearlyID:       "enterprise_yearly",
			MonthlyPricing: 9900,
			YearlyPricing:  99000,
			Features: []string{
				"Unlimited servers",
				"Unlimited sites per server",
				"Unlimited deployments",
				"Unlimited team members",
				"Server monitoring",
				"Automated backups",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            999999,
				MaxSitesPerServer:     999999,
				MaxDeploymentsPerSite: 999999,
				MaxTeamMembers:        999999,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
	}
}

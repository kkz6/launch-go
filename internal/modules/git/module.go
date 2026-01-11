package git

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/queue"
)

// Module represents the git module
type Module struct {
	handler         *Handler
	webhookHandler  *WebhookHandler
	service         *Service
	providerFactory *providers.ProviderFactory
}

// Config holds the configuration for git providers
type Config struct {
	GitHub    GitHubConfig
	GitLab    GitLabConfig
	Bitbucket BitbucketConfig
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	AppID         string
	PrivateKey    string
	WebhookSecret string
	AppSlug       string
	ClientID      string
	ClientSecret  string
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	ClientID      string
	ClientSecret  string
	WebhookSecret string
}

// BitbucketConfig holds Bitbucket-specific configuration
type BitbucketConfig struct {
	ClientID      string
	ClientSecret  string
	WebhookSecret string
}

// NewModule creates a new git module
func NewModule(db *gorm.DB, queueClient *queue.Client, logger *zerolog.Logger, config Config) *Module {
	repo := NewRepository(db)
	providerFactory := createProviderFactory(config)
	service := NewService(repo, providerFactory, queueClient, logger)
	handler := NewHandler(service)
	webhookHandler := NewWebhookHandler(service, providerFactory, logger)

	return &Module{
		handler:         handler,
		webhookHandler:  webhookHandler,
		service:         service,
		providerFactory: providerFactory,
	}
}

// createProviderFactory creates and configures the provider factory
func createProviderFactory(config Config) *providers.ProviderFactory {
	factory := providers.NewProviderFactory()

	// Register GitHub provider
	if config.GitHub.AppID != "" {
		factory.RegisterConfig(providers.GitProviderType(GitProviderGitHub), &providers.ProviderConfig{
			AppID:         config.GitHub.AppID,
			PrivateKey:    config.GitHub.PrivateKey,
			WebhookSecret: config.GitHub.WebhookSecret,
			AppSlug:       config.GitHub.AppSlug,
			ClientID:      config.GitHub.ClientID,
			ClientSecret:  config.GitHub.ClientSecret,
		})
	}

	// Register GitLab provider
	if config.GitLab.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(GitProviderGitLab), &providers.ProviderConfig{
			ClientID:      config.GitLab.ClientID,
			ClientSecret:  config.GitLab.ClientSecret,
			WebhookSecret: config.GitLab.WebhookSecret,
		})
	}

	// Register Bitbucket provider
	if config.Bitbucket.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(GitProviderBitbucket), &providers.ProviderConfig{
			ClientID:      config.Bitbucket.ClientID,
			ClientSecret:  config.Bitbucket.ClientSecret,
			WebhookSecret: config.Bitbucket.WebhookSecret,
		})
	}

	return factory
}

// RegisterRoutes registers all git routes with the router
func (m *Module) RegisterRoutesWithMiddleware(router fiber.Router, authMiddleware fiber.Handler, teamMiddleware fiber.Handler) {
	// Settings routes (authenticated + team scope)
	settings := router.Group("/settings", authMiddleware, teamMiddleware)
	{
		settings.Get("/git-providers", m.handler.GetInstallationsWithCounts)
		settings.Get("/git-providers/:provider/installation-url", m.handler.GetInstallationURL)
		settings.Get("/git-providers/:provider/installations", m.handler.GetInstallations)
		settings.Get("/git-providers/:provider/installations/:installationId/repositories", m.handler.GetInstallationRepositories)
		settings.Get("/git-providers/:provider/installations/:installationId/cached-repositories", m.handler.GetCachedInstallationRepositories)
		settings.Post("/git-providers/:provider/installations/:installationId/refresh-repositories", m.handler.RefreshInstallationRepositories)
		settings.Get("/git-providers/:provider/callback", m.handler.HandleInstallationCallback)
	}

	// App-based routes (authenticated + team scope)
	integrations := router.Group("/integrations/git-apps", authMiddleware, teamMiddleware)
	{
		integrations.Get("/:provider/installation-url", m.handler.GetInstallationURL)
		integrations.Get("/:provider/installations", m.handler.GetInstallations)
		integrations.Get("/:provider/installations/:installationId", m.handler.GetInstallation)
		integrations.Get("/:provider/installations/:installationId/repositories", m.handler.GetInstallationRepositories)
		integrations.Get("/:provider/test-connection", m.handler.TestConnection)
	}

	// Source controls CRUD (authenticated + team scope)
	sourceControls := router.Group("/source-controls", authMiddleware, teamMiddleware)
	{
		sourceControls.Get("/", m.handler.ListSourceControls)
		sourceControls.Get("/:id", m.handler.GetSourceControl)
		sourceControls.Post("/", m.handler.Connect)
		sourceControls.Delete("/:id", m.handler.Disconnect)
	}

	// Webhook routes (no auth required)
	webhooks := router.Group("/webhooks/git")
	{
		webhooks.Post("/:provider", m.webhookHandler.HandleWebhook)
	}
}

// Service returns the git service for use by other modules
func (m *Module) Service() *Service {
	return m.service
}

// ProviderFactory returns the provider factory
func (m *Module) ProviderFactory() *providers.ProviderFactory {
	return m.providerFactory
}

// AutoMigrate runs auto-migration for git models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&SourceControl{},
		&SourceControlRepository{},
	)
}

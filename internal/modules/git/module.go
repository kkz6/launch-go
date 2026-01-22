package git

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/git/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "git"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module represents the git module
type Module struct {
	app.Base

	// Repository registry
	repos *repositories.Registry

	// Provider factory (needed by other modules)
	providerFactory *providers.ProviderFactory
}

// NewModule creates a new git module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:            app.NewBase(ModuleName, b),
		repos:           repositories.NewRegistry(deps.DB),
		providerFactory: createProviderFactory(deps.Config.Git),
	}
}

// createServices creates all services needed for route handlers
func (m *Module) createServices() *services.ServiceRegistry {
	deps := m.Deps()

	// Create shared service dependencies using standardized ModuleDeps
	svcDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ProviderFactory: m.providerFactory,
	}

	// Create service registry - handles all service creation and wiring
	return services.NewServiceRegistry(svcDeps)
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}

// createProviderFactory creates and configures the provider factory
func createProviderFactory(cfg config.GitConfig) *providers.ProviderFactory {
	factory := providers.NewProviderFactory()

	// Register GitHub provider
	if cfg.GitHub.AppID != "" {
		factory.RegisterConfig(providers.GitProviderType(gittypes.GitProviderGitHub), &providers.ProviderConfig{
			AppID:         cfg.GitHub.AppID,
			PrivateKey:    cfg.GitHub.PrivateKey,
			WebhookSecret: cfg.GitHub.WebhookSecret,
			AppSlug:       cfg.GitHub.AppSlug,
		})
	}

	// Register GitLab provider
	if cfg.GitLab.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(gittypes.GitProviderGitLab), &providers.ProviderConfig{
			ClientID:      cfg.GitLab.ClientID,
			ClientSecret:  cfg.GitLab.ClientSecret,
			WebhookSecret: cfg.GitLab.WebhookSecret,
		})
	}

	// Register Bitbucket provider
	if cfg.Bitbucket.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(gittypes.GitProviderBitbucket), &providers.ProviderConfig{
			ClientID:      cfg.Bitbucket.ClientID,
			ClientSecret:  cfg.Bitbucket.ClientSecret,
			WebhookSecret: cfg.Bitbucket.WebhookSecret,
		})
	}

	return factory
}

// ProviderFactory returns the provider factory
func (m *Module) ProviderFactory() *providers.ProviderFactory {
	return m.providerFactory
}

// RegisterJobs registers background job handlers
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	// Create services for job handlers
	svc := m.createServices()

	// Register job handlers
	jobs.Register(mux, deps, m.repos, svc.SourceControl(), m.providerFactory)
}

package git

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
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
	module.Base
	scRepo          *repositories.SourceControlRepository
	repoRepo        *repositories.SourceControlRepoRepository
	service         *services.SourceControlService
	providerFactory *providers.ProviderFactory
}

// NewModule creates a new git module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	scRepo := repositories.NewSourceControlRepository(deps.DB)
	repoRepo := repositories.NewSourceControlRepoRepository(deps.DB)
	providerFactory := createProviderFactory(deps.Config.Git)
	service := services.NewSourceControlService(scRepo, repoRepo, providerFactory, deps.Queue, deps.Logger)

	return &Module{
		Base:            module.NewBase(ModuleName, b),
		scRepo:          scRepo,
		repoRepo:        repoRepo,
		service:         service,
		providerFactory: providerFactory,
	}
}

// createProviderFactory creates and configures the provider factory
func createProviderFactory(cfg config.GitConfig) *providers.ProviderFactory {
	factory := providers.NewProviderFactory()

	// Register GitHub provider
	if cfg.GitHub.AppID != "" {
		factory.RegisterConfig(providers.GitProviderType(enums.GitProviderGitHub), &providers.ProviderConfig{
			AppID:         cfg.GitHub.AppID,
			PrivateKey:    cfg.GitHub.PrivateKey,
			WebhookSecret: cfg.GitHub.WebhookSecret,
			AppSlug:       cfg.GitHub.AppSlug,
			ClientID:      cfg.GitHub.ClientID,
			ClientSecret:  cfg.GitHub.ClientSecret,
		})
	}

	// Register GitLab provider
	if cfg.GitLab.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(enums.GitProviderGitLab), &providers.ProviderConfig{
			ClientID:      cfg.GitLab.ClientID,
			ClientSecret:  cfg.GitLab.ClientSecret,
			WebhookSecret: cfg.GitLab.WebhookSecret,
		})
	}

	// Register Bitbucket provider
	if cfg.Bitbucket.ClientID != "" {
		factory.RegisterConfig(providers.GitProviderType(enums.GitProviderBitbucket), &providers.ProviderConfig{
			ClientID:      cfg.Bitbucket.ClientID,
			ClientSecret:  cfg.Bitbucket.ClientSecret,
			WebhookSecret: cfg.Bitbucket.WebhookSecret,
		})
	}

	return factory
}

// Service returns the git service for use by other modules
func (m *Module) Service() *services.SourceControlService {
	return m.service
}

// ProviderFactory returns the provider factory
func (m *Module) ProviderFactory() *providers.ProviderFactory {
	return m.providerFactory
}

// RegisterJobs registers background job handlers
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	// Set up job context
	jobContext := jobs.NewJobContext(
		deps.DB,
		deps.Logger,
		m.service,
		m.providerFactory,
		m.scRepo,
		deps.Queue,
	)
	jobs.SetJobContext(jobContext)

	// Register job handlers
	jobs.RegisterHandlers(mux)
}

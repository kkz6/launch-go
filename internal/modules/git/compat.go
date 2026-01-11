package git

import (
	"context"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	"github.com/kkz6/launch-go/internal/queue"
)

// Repository is a compatibility alias that combines both repository types
// for backward compatibility with existing code and tests.
type Repository struct {
	db *gorm.DB
	*repositories.SourceControlRepository
	*repositories.SourceControlRepoRepository
}

// NewRepository creates a repository wrapper for backward compatibility.
// This creates both the SourceControlRepository and SourceControlRepoRepository
// and wraps them in a single struct.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:                          db,
		SourceControlRepository:     repositories.NewSourceControlRepository(db),
		SourceControlRepoRepository: repositories.NewSourceControlRepoRepository(db),
	}
}

// NewService creates a new SourceControlService for backward compatibility.
// It accepts a *Repository and other dependencies.
func NewService(
	repo *Repository,
	providerFactory *providers.ProviderFactory,
	queue *queue.Client,
	logger *zerolog.Logger,
) *Service {
	return services.NewSourceControlService(
		repo.SourceControlRepository,
		repo.SourceControlRepoRepository,
		providerFactory,
		queue,
		logger,
	)
}

// Delegated methods for Repository to maintain full backward compatibility
// These delegate to the embedded SourceControlRepoRepository

// CreateRepository creates a new source control repository
func (r *Repository) CreateRepository(ctx context.Context, repo *models.SourceControlRepository) error {
	return r.SourceControlRepoRepository.CreateRepository(ctx, repo)
}

// UpdateRepository updates an existing repository
func (r *Repository) UpdateRepository(ctx context.Context, repo *models.SourceControlRepository) error {
	return r.SourceControlRepoRepository.UpdateRepository(ctx, repo)
}

// DeleteRepository deletes a repository
func (r *Repository) DeleteRepository(ctx context.Context, id string) error {
	return r.SourceControlRepoRepository.DeleteRepository(ctx, id)
}

// DeleteRepositoriesBySourceControlID deletes all repositories for a source control
func (r *Repository) DeleteRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) error {
	return r.SourceControlRepoRepository.DeleteRepositoriesBySourceControlID(ctx, sourceControlID)
}

// DeleteRepositoriesByIDs deletes repositories by their IDs
func (r *Repository) DeleteRepositoriesByIDs(ctx context.Context, ids []string) error {
	return r.SourceControlRepoRepository.DeleteRepositoriesByIDs(ctx, ids)
}

// FindRepositoryByID finds a repository by ID
func (r *Repository) FindRepositoryByID(ctx context.Context, id string) (*models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.FindRepositoryByID(ctx, id)
}

// FindRepositoryByFullName finds a repository by its full name
func (r *Repository) FindRepositoryByFullName(ctx context.Context, fullName string) (*models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.FindRepositoryByFullName(ctx, fullName)
}

// FindRepositoriesBySourceControlID finds all repositories for a source control
func (r *Repository) FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.FindRepositoriesBySourceControlID(ctx, sourceControlID)
}

// FindPublicRepositories finds all public repositories
func (r *Repository) FindPublicRepositories(ctx context.Context) ([]models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.FindPublicRepositories(ctx)
}

// GetInstallationRepositories gets repositories for a specific installation
func (r *Repository) GetInstallationRepositories(ctx context.Context, provider enums.GitProviderType, installationID string, teamID string) ([]models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.GetInstallationRepositories(ctx, provider, installationID, teamID)
}

// UpsertRepository creates or updates a repository
func (r *Repository) UpsertRepository(ctx context.Context, sourceControlID string, data *dto.RepositoryData) (*models.SourceControlRepository, error) {
	return r.SourceControlRepoRepository.UpsertRepository(ctx, sourceControlID, data)
}

// CountRepositoriesBySourceControlID counts repositories for a source control
func (r *Repository) CountRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) (int64, error) {
	return r.SourceControlRepoRepository.CountRepositoriesBySourceControlID(ctx, sourceControlID)
}

// Delegated methods for SourceControl

// Create creates a new source control record
func (r *Repository) Create(ctx context.Context, sc *models.SourceControl) error {
	return r.SourceControlRepository.Create(ctx, sc)
}

// Update updates an existing source control record
func (r *Repository) Update(ctx context.Context, sc *models.SourceControl) error {
	return r.SourceControlRepository.Update(ctx, sc)
}

// UpdateFields updates specific fields of a source control record
func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.SourceControlRepository.UpdateFields(ctx, id, fields)
}

// Delete soft-deletes a source control record
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.SourceControlRepository.Delete(ctx, id)
}

// FindByID finds a source control by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*models.SourceControl, error) {
	return r.SourceControlRepository.FindByID(ctx, id)
}

// FindByIDAndTeam finds a source control by ID and team ID
func (r *Repository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.SourceControl, error) {
	return r.SourceControlRepository.FindByIDAndTeam(ctx, id, teamID)
}

// FindAllByTeam finds all source controls for a team
func (r *Repository) FindAllByTeam(ctx context.Context, teamID string) ([]models.SourceControl, error) {
	return r.SourceControlRepository.FindAllByTeam(ctx, teamID)
}

// FindAllByUser finds all source controls for a user
func (r *Repository) FindAllByUser(ctx context.Context, userID string) ([]models.SourceControl, error) {
	return r.SourceControlRepository.FindAllByUser(ctx, userID)
}

// FindByProvider finds all source controls for a specific provider
func (r *Repository) FindByProvider(ctx context.Context, provider enums.GitProviderType) ([]models.SourceControl, error) {
	return r.SourceControlRepository.FindByProvider(ctx, provider)
}

// FindByTeamAndProvider finds source controls for a team and provider
func (r *Repository) FindByTeamAndProvider(ctx context.Context, teamID string, provider enums.GitProviderType) ([]models.SourceControl, error) {
	return r.SourceControlRepository.FindByTeamAndProvider(ctx, teamID, provider)
}

// FindByProviderAndInstallationAndTeam finds a source control by provider, installation ID, and team
func (r *Repository) FindByProviderAndInstallationAndTeam(ctx context.Context, provider enums.GitProviderType, installationID string, teamID string) (*models.SourceControl, error) {
	return r.SourceControlRepository.FindByProviderAndInstallationAndTeam(ctx, provider, installationID, teamID)
}

// FirstOrCreateByProviderAndInstallationAndTeam finds or creates a source control
func (r *Repository) FirstOrCreateByProviderAndInstallationAndTeam(ctx context.Context, provider enums.GitProviderType, installationID string, teamID string, defaults map[string]interface{}) (*models.SourceControl, bool, error) {
	return r.SourceControlRepository.FirstOrCreateByProviderAndInstallationAndTeam(ctx, provider, installationID, teamID, defaults)
}

// FindByInstallationID finds all source controls by installation ID
func (r *Repository) FindByInstallationID(ctx context.Context, installationID string) ([]models.SourceControl, error) {
	return r.SourceControlRepository.FindByInstallationID(ctx, installationID)
}

// DeleteByInstallationID deletes all source controls and their repositories by installation ID
func (r *Repository) DeleteByInstallationID(ctx context.Context, installationID string) (int64, error) {
	return r.SourceControlRepository.DeleteByInstallationID(ctx, installationID)
}

// GetInstallations gets installations with flexible filtering
func (r *Repository) GetInstallations(ctx context.Context, provider enums.GitProviderType, opts ...contracts.InstallationQueryOption) ([]models.SourceControl, error) {
	return r.SourceControlRepository.GetInstallations(ctx, provider, opts...)
}

// GetFirstInstallation gets the first installation matching criteria
func (r *Repository) GetFirstInstallation(ctx context.Context, provider enums.GitProviderType, opts ...contracts.InstallationQueryOption) (*models.SourceControl, error) {
	return r.SourceControlRepository.GetFirstInstallation(ctx, provider, opts...)
}

// parseCommitter parses a raw author string like "John Doe <john@example.com>"
// This is exported for backward compatibility with existing tests.
func parseCommitter(raw string) (name, email string) {
	re := regexp.MustCompile(`^(.*?)\s*<(.+?)>$`)
	matches := re.FindStringSubmatch(raw)

	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), matches[2]
	}

	return "", ""
}

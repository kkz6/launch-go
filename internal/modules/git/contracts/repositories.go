package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// SourceControlRepository defines the interface for source control database operations
type SourceControlRepository interface {
	Create(ctx context.Context, sc *models.SourceControl) error
	Update(ctx context.Context, sc *models.SourceControl) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*models.SourceControl, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.SourceControl, error)
	FindAllByTeam(ctx context.Context, teamID string) ([]models.SourceControl, error)
	FindAllByUser(ctx context.Context, userID string) ([]models.SourceControl, error)
	FindByProvider(ctx context.Context, provider gittypes.GitProviderType) ([]models.SourceControl, error)
	FindByTeamAndProvider(ctx context.Context, teamID string, provider gittypes.GitProviderType) ([]models.SourceControl, error)
	FindByProviderAndInstallationAndTeam(ctx context.Context, provider gittypes.GitProviderType, installationID string, teamID string) (*models.SourceControl, error)
	FirstOrCreateByProviderAndInstallationAndTeam(ctx context.Context, provider gittypes.GitProviderType, installationID string, teamID string, defaults map[string]interface{}) (*models.SourceControl, bool, error)
	FindByInstallationID(ctx context.Context, installationID string) ([]models.SourceControl, error)
	DeleteByInstallationID(ctx context.Context, installationID string) (int64, error)
	GetInstallations(ctx context.Context, provider gittypes.GitProviderType, opts ...InstallationQueryOption) ([]models.SourceControl, error)
	GetFirstInstallation(ctx context.Context, provider gittypes.GitProviderType, opts ...InstallationQueryOption) (*models.SourceControl, error)
}

// SourceControlRepoRepository defines the interface for source control repository database operations
type SourceControlRepoRepository interface {
	CreateRepository(ctx context.Context, repo *models.SourceControlRepository) error
	UpdateRepository(ctx context.Context, repo *models.SourceControlRepository) error
	DeleteRepository(ctx context.Context, id string) error
	DeleteRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) error
	DeleteRepositoriesByIDs(ctx context.Context, ids []string) error
	FindRepositoryByID(ctx context.Context, id string) (*models.SourceControlRepository, error)
	FindRepositoryByFullName(ctx context.Context, fullName string) (*models.SourceControlRepository, error)
	FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]models.SourceControlRepository, error)
	FindPublicRepositories(ctx context.Context) ([]models.SourceControlRepository, error)
	GetInstallationRepositories(ctx context.Context, provider gittypes.GitProviderType, installationID string, teamID string) ([]models.SourceControlRepository, error)
	UpsertRepository(ctx context.Context, sourceControlID string, data *dto.RepositoryData) (*models.SourceControlRepository, error)
	CountRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) (int64, error)
}

// InstallationQueryOptions holds options for installation queries
type InstallationQueryOptions struct {
	UserID                string
	ProviderID            string
	RequireInstallationID bool
}

// InstallationQueryOption is a functional option for installation queries
type InstallationQueryOption func(*InstallationQueryOptions)

// WithUserID filters installations by user ID
func WithUserID(userID string) InstallationQueryOption {
	return func(o *InstallationQueryOptions) {
		o.UserID = userID
	}
}

// WithProviderID filters installations by provider ID
func WithProviderID(providerID string) InstallationQueryOption {
	return func(o *InstallationQueryOptions) {
		o.ProviderID = providerID
	}
}

// RequireInstallationID requires that installation_id is not null
func RequireInstallationID() InstallationQueryOption {
	return func(o *InstallationQueryOptions) {
		o.RequireInstallationID = true
	}
}

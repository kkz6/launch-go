package contracts

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// SourceControlRepository defines the interface for source control database operations
type SourceControlRepository interface {
	Create(ctx context.Context, sc *models.SourceControl) error
	Update(ctx context.Context, sc *models.SourceControl) error
	UpdateFields(ctx context.Context, id string, updates SourceControlUpdates) error
	DeleteWithRepositories(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*models.SourceControl, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.SourceControl, error)
	FindAllByTeam(ctx context.Context, teamID string) ([]models.SourceControl, error)
	FindByTeamAndProvider(ctx context.Context, teamID string, provider gittypes.GitProviderType) ([]models.SourceControl, error)
	FindByProviderAndInstallationAndTeam(ctx context.Context, provider gittypes.GitProviderType, installationID string, teamID string) (*models.SourceControl, error)
	FindByInstallationID(ctx context.Context, installationID string) ([]models.SourceControl, error)
	DeleteByInstallationID(ctx context.Context, provider gittypes.GitProviderType, installationID string) (int64, error)
	GetInstallations(ctx context.Context, provider gittypes.GitProviderType, opts ...InstallationQueryOption) ([]models.SourceControl, error)
	GetFirstInstallation(ctx context.Context, provider gittypes.GitProviderType, opts ...InstallationQueryOption) (*models.SourceControl, error)
}

// SourceControlUpdates contains optional source-control columns to update.
type SourceControlUpdates struct {
	RepositoryCount *int
	LastSyncedAt    *time.Time
	ProviderData    *string
}

// SourceControlRepoRepository defines the interface for source control repository database operations
type SourceControlRepoRepository interface {
	DeleteRepositoriesByIDs(ctx context.Context, ids []string) error
	FindRepositoryByID(ctx context.Context, id string) (*models.SourceControlRepository, error)
	FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]models.SourceControlRepository, error)
	GetInstallationRepositories(ctx context.Context, provider gittypes.GitProviderType, installationID string, teamID string) ([]models.SourceControlRepository, error)
	UpsertRepository(ctx context.Context, sourceControlID string, data *dto.RepositoryData) (*models.SourceControlRepository, error)
}

// InstallationQueryOptions holds options for installation queries
type InstallationQueryOptions struct {
	UserID                string
	TeamID                string
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

// WithTeamID filters installations by team ID
func WithTeamID(teamID string) InstallationQueryOption {
	return func(o *InstallationQueryOptions) {
		o.TeamID = teamID
	}
}

// RequireInstallationID requires that installation_id is not null
func RequireInstallationID() InstallationQueryOption {
	return func(o *InstallationQueryOptions) {
		o.RequireInstallationID = true
	}
}

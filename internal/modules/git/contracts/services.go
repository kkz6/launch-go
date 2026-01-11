package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
)

// SourceControlService defines the interface for source control business logic
type SourceControlService interface {
	ListSourceControls(ctx context.Context, teamID string) ([]models.SourceControl, error)
	GetSourceControl(ctx context.Context, id, teamID string) (*models.SourceControl, error)
	Connect(ctx context.Context, userID, teamID string, providerType enums.GitProviderType, installationID string) (*models.SourceControl, error)
	Disconnect(ctx context.Context, id, teamID string) error
	GetInstallationURL(providerType enums.GitProviderType) (string, error)
	GetInstallations(ctx context.Context, providerType enums.GitProviderType) ([]dto.AppInstallationData, error)
	GetInstallationRepositoriesFromAPI(ctx context.Context, providerType enums.GitProviderType, installationID string) ([]map[string]interface{}, error)
	GetCachedRepositories(ctx context.Context, providerType enums.GitProviderType, installationID, teamID string) ([]models.SourceControlRepository, error)
	SyncRepositories(ctx context.Context, sc *models.SourceControl) error
	SyncInstallationRepositories(ctx context.Context, sc *models.SourceControl, repositories []map[string]interface{}) error
	RefreshInstallationRepositories(ctx context.Context, sc *models.SourceControl) error
	SyncUserInstallation(ctx context.Context, providerType enums.GitProviderType, installationID, teamID, userID string) error
	SyncRepositoriesForInstallation(ctx context.Context, installationID string) error
	GetInstallationsWithRepositoryCounts(ctx context.Context, teamID, userID string) (map[string][]dto.InstallationSummaryData, error)
	DeleteByInstallationID(ctx context.Context, installationID string) error
	GetSourceControlByInstallation(ctx context.Context, providerType enums.GitProviderType, installationID string, opts ...InstallationQueryOption) (*models.SourceControl, error)
	TestConnection(ctx context.Context, providerType enums.GitProviderType) error
}

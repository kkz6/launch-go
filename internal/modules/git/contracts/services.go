package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// SourceControlService defines the interface for source control business logic
type SourceControlService interface {
	ListSourceControls(ctx context.Context, teamID string) ([]dto.SourceControlResponse, error)
	GetSourceControl(ctx context.Context, id, teamID string) (dto.SourceControlResponse, error)
	Connect(ctx context.Context, teamID, userID string, req *dto.ConnectProviderRequest) (dto.SourceControlResponse, error)
	Disconnect(ctx context.Context, id, teamID, userID string) error
	GetInstallationURL(providerType gittypes.GitProviderType) (string, error)
	ListSourceControlsByProvider(ctx context.Context, providerType gittypes.GitProviderType, teamID string) ([]models.SourceControl, error)
	GetCachedRepositories(ctx context.Context, providerType gittypes.GitProviderType, installationID, teamID string) ([]models.SourceControlRepository, error)
	SyncRepositories(ctx context.Context, sc *models.SourceControl) error
	SyncInstallationRepositories(ctx context.Context, sc *models.SourceControl, repositories []map[string]interface{}) error
	RefreshInstallationRepositories(ctx context.Context, sc *models.SourceControl) error
	SyncUserInstallation(ctx context.Context, providerType gittypes.GitProviderType, installationID, teamID, userID string) error
	SyncRepositoriesForInstallation(ctx context.Context, installationID string) error
	GetInstallationsWithRepositoryCounts(ctx context.Context, teamID string) (map[string][]dto.InstallationSummaryData, error)
	DeleteByInstallationID(ctx context.Context, provider gittypes.GitProviderType, installationID string) error
	GetSourceControlByInstallation(ctx context.Context, providerType gittypes.GitProviderType, installationID string, opts ...InstallationQueryOption) (*models.SourceControl, error)
	TestConnection(ctx context.Context, providerType gittypes.GitProviderType) error
	// SaveRepository resolves an existing explicit selection; it never imports a grant.
	SaveRepository(ctx context.Context, sourceControlID, teamID, repoFullName string) (*models.SourceControlRepository, error)
}

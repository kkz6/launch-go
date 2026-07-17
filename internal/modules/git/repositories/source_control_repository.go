package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SourceControlRepository handles database operations for SourceControl entities
type SourceControlRepository struct {
	repository.Base[models.SourceControl]
}

// NewSourceControlRepository creates a new source control repository
func NewSourceControlRepository(db *gorm.DB) *SourceControlRepository {
	return &SourceControlRepository{
		Base: repository.NewBase[models.SourceControl](db),
	}
}

// Create creates a new source control record
func (r *SourceControlRepository) Create(ctx context.Context, sc *models.SourceControl) error {
	return r.DB.WithContext(ctx).Create(sc).Error
}

// Update updates an existing source control record
func (r *SourceControlRepository) Update(ctx context.Context, sc *models.SourceControl) error {
	return r.DB.WithContext(ctx).
		Model(&models.SourceControl{}).
		Where("id = ?", sc.ID).
		Updates(map[string]any{
			"user_id":                   sc.UserID,
			"team_id":                   sc.TeamID,
			"provider_id":               sc.ProviderID,
			"provider_account_id":       sc.ProviderAccountID,
			"login":                     sc.Login,
			"name":                      sc.Name,
			"type":                      sc.Type,
			"avatar_url":                sc.AvatarURL,
			"html_url":                  sc.HTMLURL,
			"installation_id":           sc.InstallationID,
			"permissions":               sc.Permissions,
			"repository_selection":      sc.RepositorySelection,
			"has_multiple_repositories": sc.HasMultipleRepositories,
			"repository_count":          sc.RepositoryCount,
			"connected_at":              sc.ConnectedAt,
			"last_synced_at":            sc.LastSyncedAt,
			"additional_data":           sc.AdditionalData,
			"provider":                  sc.Provider,
			"url":                       sc.URL,
			"provider_data":             sc.ProviderData,
			"token_expires_at":          sc.TokenExpiresAt,
		}).Error
}

// UpdateFields updates specific fields of a source control record
func (r *SourceControlRepository) UpdateFields(ctx context.Context, id string, updates contracts.SourceControlUpdates) error {
	fields := make(map[string]any, 3)
	if updates.RepositoryCount != nil {
		fields["repository_count"] = *updates.RepositoryCount
	}
	if updates.LastSyncedAt != nil {
		fields["last_synced_at"] = *updates.LastSyncedAt
	}
	if updates.ProviderData != nil {
		fields["provider_data"] = *updates.ProviderData
	}
	if len(fields) == 0 {
		return nil
	}

	return r.DB.WithContext(ctx).
		Model(&models.SourceControl{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete soft-deletes a source control record
func (r *SourceControlRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.SourceControl{}, "id = ?", id).Error
}

// FindByID finds a source control by ID
func (r *SourceControlRepository) FindByID(ctx context.Context, id string) (*models.SourceControl, error) {
	return repository.FindOne[models.SourceControl](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Repositories"),
	)
}

// FindByIDAndTeam finds a source control by ID and team ID
func (r *SourceControlRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.SourceControl, error) {
	return repository.FindOne[models.SourceControl](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
		repository.Preload("Repositories"),
	)
}

// FindAllByTeam finds all source controls for a team
func (r *SourceControlRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByTeamAndProvider finds source controls for a team and provider
func (r *SourceControlRepository) FindByTeamAndProvider(ctx context.Context, teamID string, provider gittypes.GitProviderType) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("team_id = ? AND provider = ? AND installation_id IS NOT NULL", teamID, provider).
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByProviderAndInstallationAndTeam finds a source control by provider, installation ID, and team
func (r *SourceControlRepository) FindByProviderAndInstallationAndTeam(
	ctx context.Context,
	provider gittypes.GitProviderType,
	installationID string,
	teamID string,
) (*models.SourceControl, error) {
	var sc models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fiberutil.NotFound()
	}

	return &sc, err
}

// FindByInstallationID finds all source controls by installation ID
func (r *SourceControlRepository) FindByInstallationID(ctx context.Context, installationID string) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider_id = ?", installationID).
		Find(&sourceControls).Error

	return sourceControls, err
}

// DeleteByInstallationID deletes all source controls and their repositories by installation ID
func (r *SourceControlRepository) DeleteByInstallationID(
	ctx context.Context,
	provider gittypes.GitProviderType,
	installationID string,
) (int64, error) {
	var deleted int64
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sourceControlIDs []string
		if err := tx.Model(&models.SourceControl{}).
			Where("provider = ? AND provider_id = ?", provider, installationID).
			Pluck("id", &sourceControlIDs).Error; err != nil {
			return err
		}

		if len(sourceControlIDs) == 0 {
			return nil
		}

		if err := tx.Where("source_control_id IN ?", sourceControlIDs).
			Delete(&models.SourceControlRepository{}).Error; err != nil {
			return err
		}

		result := tx.Where("provider = ? AND provider_id = ?", provider, installationID).
			Delete(&models.SourceControl{})
		deleted = result.RowsAffected

		return result.Error
	})

	return deleted, err
}

// GetInstallations gets installations with flexible filtering
func (r *SourceControlRepository) GetInstallations(
	ctx context.Context,
	provider gittypes.GitProviderType,
	opts ...contracts.InstallationQueryOption,
) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.installationsQuery(ctx, provider, opts...).Find(&sourceControls).Error

	return sourceControls, err
}

// GetFirstInstallation gets the first installation matching criteria
func (r *SourceControlRepository) GetFirstInstallation(
	ctx context.Context,
	provider gittypes.GitProviderType,
	opts ...contracts.InstallationQueryOption,
) (*models.SourceControl, error) {
	var sc models.SourceControl
	err := r.installationsQuery(ctx, provider, opts...).First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fiberutil.NotFound()
	}

	return &sc, err
}

func (r *SourceControlRepository) installationsQuery(
	ctx context.Context,
	provider gittypes.GitProviderType,
	opts ...contracts.InstallationQueryOption,
) *gorm.DB {
	options := &contracts.InstallationQueryOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider)

	if options.UserID != "" {
		query = query.Where("user_id = ?", options.UserID)
	}
	if options.TeamID != "" {
		query = query.Where("team_id = ?", options.TeamID)
	}
	if options.ProviderID != "" {
		query = query.Where("provider_id = ?", options.ProviderID)
	}
	if options.RequireInstallationID {
		query = query.Where("installation_id IS NOT NULL")
	}

	return query
}

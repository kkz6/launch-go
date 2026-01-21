package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
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
	return r.DB.WithContext(ctx).Save(sc).Error
}

// UpdateFields updates specific fields of a source control record
func (r *SourceControlRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
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
	var sc models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		First(&sc, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// FindByIDAndTeam finds a source control by ID and team ID
func (r *SourceControlRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.SourceControl, error) {
	var sc models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		First(&sc, "id = ? AND team_id = ?", id, teamID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
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

// FindAllByUser finds all source controls for a user
func (r *SourceControlRepository) FindAllByUser(ctx context.Context, userID string) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByProvider finds all source controls for a specific provider
func (r *SourceControlRepository) FindByProvider(ctx context.Context, provider enums.GitProviderType) ([]models.SourceControl, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider).
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByTeamAndProvider finds source controls for a team and provider
func (r *SourceControlRepository) FindByTeamAndProvider(ctx context.Context, teamID string, provider enums.GitProviderType) ([]models.SourceControl, error) {
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
	provider enums.GitProviderType,
	installationID string,
	teamID string,
) (*models.SourceControl, error) {
	var sc models.SourceControl
	err := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// FirstOrCreateByProviderAndInstallationAndTeam finds or creates a source control
func (r *SourceControlRepository) FirstOrCreateByProviderAndInstallationAndTeam(
	ctx context.Context,
	provider enums.GitProviderType,
	installationID string,
	teamID string,
	defaults map[string]interface{},
) (*models.SourceControl, bool, error) {
	var sc models.SourceControl
	err := r.DB.WithContext(ctx).
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		First(&sc).Error

	if err == nil {
		return &sc, false, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// Create new record
	sc = models.SourceControl{
		Provider:   provider,
		ProviderID: installationID,
		TeamID:     teamID,
	}

	// Apply defaults
	if userID, ok := defaults["user_id"].(string); ok {
		sc.UserID = userID
	}

	if providerData, ok := defaults["provider_data"].(models.JSONMap); ok {
		if jsonBytes, err := json.Marshal(providerData); err == nil {
			jsonStr := string(jsonBytes)
			sc.ProviderData = &jsonStr
		}
	}

	if err := r.DB.WithContext(ctx).Create(&sc).Error; err != nil {
		return nil, false, err
	}

	return &sc, true, nil
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
func (r *SourceControlRepository) DeleteByInstallationID(ctx context.Context, installationID string) (int64, error) {
	// First, get all source control IDs
	var sourceControls []models.SourceControl
	if err := r.DB.WithContext(ctx).
		Select("id").
		Where("provider_id = ?", installationID).
		Find(&sourceControls).Error; err != nil {
		return 0, err
	}

	if len(sourceControls) == 0 {
		return 0, nil
	}

	ids := make([]string, len(sourceControls))
	for i, sc := range sourceControls {
		ids[i] = sc.ID
	}

	// Delete repositories
	if err := r.DB.WithContext(ctx).
		Where("source_control_id IN ?", ids).
		Delete(&models.SourceControlRepository{}).Error; err != nil {
		return 0, err
	}

	// Delete source controls
	result := r.DB.WithContext(ctx).
		Where("provider_id = ?", installationID).
		Delete(&models.SourceControl{})

	return result.RowsAffected, result.Error
}

// GetInstallations gets installations with flexible filtering
func (r *SourceControlRepository) GetInstallations(
	ctx context.Context,
	provider enums.GitProviderType,
	opts ...contracts.InstallationQueryOption,
) ([]models.SourceControl, error) {
	query := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider)

	options := &contracts.InstallationQueryOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.UserID != "" {
		query = query.Where("user_id = ?", options.UserID)
	}

	if options.ProviderID != "" {
		query = query.Where("provider_id = ?", options.ProviderID)
	}

	if options.RequireInstallationID {
		query = query.Where("installation_id IS NOT NULL")
	}

	var sourceControls []models.SourceControl
	err := query.Find(&sourceControls).Error

	return sourceControls, err
}

// GetFirstInstallation gets the first installation matching criteria
func (r *SourceControlRepository) GetFirstInstallation(
	ctx context.Context,
	provider enums.GitProviderType,
	opts ...contracts.InstallationQueryOption,
) (*models.SourceControl, error) {
	query := r.DB.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider)

	options := &contracts.InstallationQueryOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.UserID != "" {
		query = query.Where("user_id = ?", options.UserID)
	}

	if options.ProviderID != "" {
		query = query.Where("provider_id = ?", options.ProviderID)
	}

	if options.RequireInstallationID {
		query = query.Where("installation_id IS NOT NULL")
	}

	var sc models.SourceControl
	err := query.First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

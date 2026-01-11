package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
)

// SourceControlRepoRepository handles database operations for SourceControlRepository entities
type SourceControlRepoRepository struct {
	db *gorm.DB
}

// NewSourceControlRepoRepository creates a new source control repo repository
func NewSourceControlRepoRepository(db *gorm.DB) *SourceControlRepoRepository {
	return &SourceControlRepoRepository{db: db}
}

// CreateRepository creates a new source control repository
func (r *SourceControlRepoRepository) CreateRepository(ctx context.Context, repo *models.SourceControlRepository) error {
	return r.db.WithContext(ctx).Create(repo).Error
}

// UpdateRepository updates an existing repository
func (r *SourceControlRepoRepository) UpdateRepository(ctx context.Context, repo *models.SourceControlRepository) error {
	return r.db.WithContext(ctx).Save(repo).Error
}

// DeleteRepository deletes a repository
func (r *SourceControlRepoRepository) DeleteRepository(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.SourceControlRepository{}, "id = ?", id).Error
}

// DeleteRepositoriesBySourceControlID deletes all repositories for a source control
func (r *SourceControlRepoRepository) DeleteRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) error {
	return r.db.WithContext(ctx).
		Where("source_control_id = ?", sourceControlID).
		Delete(&models.SourceControlRepository{}).Error
}

// DeleteRepositoriesByIDs deletes repositories by their IDs
func (r *SourceControlRepoRepository) DeleteRepositoriesByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&models.SourceControlRepository{}).Error
}

// FindRepositoryByID finds a repository by ID
func (r *SourceControlRepoRepository) FindRepositoryByID(ctx context.Context, id string) (*models.SourceControlRepository, error) {
	var repo models.SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		First(&repo, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRepositoryNotFound
	}

	return &repo, err
}

// FindRepositoryByFullName finds a repository by its full name
func (r *SourceControlRepoRepository) FindRepositoryByFullName(ctx context.Context, fullName string) (*models.SourceControlRepository, error) {
	var repo models.SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		First(&repo, "full_name = ?", fullName).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRepositoryNotFound
	}

	return &repo, err
}

// FindRepositoriesBySourceControlID finds all repositories for a source control
func (r *SourceControlRepoRepository) FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]models.SourceControlRepository, error) {
	var repos []models.SourceControlRepository
	err := r.db.WithContext(ctx).
		Where("source_control_id = ?", sourceControlID).
		Order("full_name ASC").
		Find(&repos).Error

	return repos, err
}

// FindPublicRepositories finds all public repositories
func (r *SourceControlRepoRepository) FindPublicRepositories(ctx context.Context) ([]models.SourceControlRepository, error) {
	var repos []models.SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		Where("public = ?", true).
		Find(&repos).Error

	return repos, err
}

// GetInstallationRepositories gets repositories for a specific installation
func (r *SourceControlRepoRepository) GetInstallationRepositories(
	ctx context.Context,
	provider enums.GitProviderType,
	installationID string,
	teamID string,
) ([]models.SourceControlRepository, error) {
	var sourceControls []models.SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		Find(&sourceControls).Error

	if err != nil {
		return nil, err
	}

	var repos []models.SourceControlRepository
	for _, sc := range sourceControls {
		repos = append(repos, sc.Repositories...)
	}

	return repos, nil
}

// UpsertRepository creates or updates a repository
func (r *SourceControlRepoRepository) UpsertRepository(ctx context.Context, sourceControlID string, data *dto.RepositoryData) (*models.SourceControlRepository, error) {
	var repo models.SourceControlRepository
	err := r.db.WithContext(ctx).
		Where("source_control_id = ? AND full_name = ?", sourceControlID, data.FullName).
		First(&repo).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Convert AdditionalData to *string
		var additionalData *string
		if data.AdditionalData != nil {
			if jsonBytes, err := json.Marshal(data.AdditionalData); err == nil {
				jsonStr := string(jsonBytes)
				additionalData = &jsonStr
			}
		}

		// Create new
		repo = models.SourceControlRepository{
			SourceControlID: sourceControlID,
			Name:            data.Name,
			FullName:        data.FullName,
			Public:          data.IsPublic,
			DefaultBranch:   data.DefaultBranch,
			HTMLURL:         &data.HTMLURL,
			SSHURL:          data.SSHURL,
			AdditionalData:  additionalData,
		}

		if err := r.db.WithContext(ctx).Create(&repo).Error; err != nil {
			return nil, err
		}

		return &repo, nil
	}

	if err != nil {
		return nil, err
	}

	// Convert AdditionalData to *string
	var additionalData *string
	if data.AdditionalData != nil {
		if jsonBytes, err := json.Marshal(data.AdditionalData); err == nil {
			jsonStr := string(jsonBytes)
			additionalData = &jsonStr
		}
	}

	// Update existing
	repo.Name = data.Name
	repo.Public = data.IsPublic
	repo.DefaultBranch = data.DefaultBranch
	repo.HTMLURL = &data.HTMLURL
	repo.SSHURL = data.SSHURL
	repo.AdditionalData = additionalData

	if err := r.db.WithContext(ctx).Save(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// CountRepositoriesBySourceControlID counts repositories for a source control
func (r *SourceControlRepoRepository) CountRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.SourceControlRepository{}).
		Where("source_control_id = ?", sourceControlID).
		Count(&count).Error

	return count, err
}

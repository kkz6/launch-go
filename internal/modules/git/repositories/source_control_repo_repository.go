package repositories

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SourceControlRepoRepository handles database operations for SourceControlRepository entities
type SourceControlRepoRepository struct {
	repository.Base[models.SourceControlRepository]
}

// NewSourceControlRepoRepository creates a new source control repo repository
func NewSourceControlRepoRepository(db *gorm.DB) *SourceControlRepoRepository {
	return &SourceControlRepoRepository{
		Base: repository.NewBase[models.SourceControlRepository](db),
	}
}

// DeleteRepositoriesByIDs deletes repositories by their IDs
func (r *SourceControlRepoRepository) DeleteRepositoriesByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	return r.DB.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&models.SourceControlRepository{}).Error
}

// FindRepositoryByID finds a repository by ID.
func (r *SourceControlRepoRepository) FindRepositoryByID(ctx context.Context, id string) (*models.SourceControlRepository, error) {
	return repository.FindOne[models.SourceControlRepository](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("SourceControl"),
	)
}

// FindRepositoriesBySourceControlID finds all repositories for a source control
func (r *SourceControlRepoRepository) FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]models.SourceControlRepository, error) {
	var repos []models.SourceControlRepository
	err := r.DB.WithContext(ctx).
		Where("source_control_id = ?", sourceControlID).
		Order("full_name ASC").
		Find(&repos).Error

	return repos, err
}

// GetInstallationRepositories gets repositories for a specific installation
func (r *SourceControlRepoRepository) GetInstallationRepositories(
	ctx context.Context,
	provider gittypes.GitProviderType,
	installationID string,
	teamID string,
) ([]models.SourceControlRepository, error) {
	var sourceControls []models.SourceControl
	err := r.DB.WithContext(ctx).
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
	var additionalData *string
	if data.AdditionalData != nil {
		jsonBytes, err := json.Marshal(data.AdditionalData)
		if err != nil {
			return nil, err
		}
		jsonString := string(jsonBytes)
		additionalData = &jsonString
	}

	repo := models.SourceControlRepository{
		SourceControlID: sourceControlID,
		Name:            data.Name,
		FullName:        data.FullName,
		Public:          data.IsPublic,
		DefaultBranch:   data.DefaultBranch,
		HTMLURL:         &data.HTMLURL,
		SSHURL:          data.SSHURL,
		AdditionalData:  additionalData,
	}

	err := r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "source_control_id"}, {Name: "full_name"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name",
				"public",
				"default_branch",
				"html_url",
				"ssh_url",
				"additional_data",
			}),
		}).
		Create(&repo).Error
	if err != nil {
		return nil, err
	}

	if err := r.DB.WithContext(ctx).
		Where("source_control_id = ? AND full_name = ?", sourceControlID, data.FullName).
		First(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

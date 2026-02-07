package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerRegistryRepository handles database operations for docker registries
type DockerRegistryRepository struct {
	db *gorm.DB
}

// NewDockerRegistryRepository creates a new DockerRegistryRepository
func NewDockerRegistryRepository(db *gorm.DB) *DockerRegistryRepository {
	return &DockerRegistryRepository{db: db}
}

// Create inserts a new docker registry record
func (r *DockerRegistryRepository) Create(ctx context.Context, registry *models.DockerRegistry) error {
	return r.db.WithContext(ctx).Create(registry).Error
}

// FindByID retrieves a docker registry by ID
func (r *DockerRegistryRepository) FindByID(ctx context.Context, id string) (*models.DockerRegistry, error) {
	var reg models.DockerRegistry

	err := r.db.WithContext(ctx).First(&reg, "id = ?", id).Error

	return &reg, err
}

// FindByTeamID retrieves all docker registries for a team
func (r *DockerRegistryRepository) FindByTeamID(ctx context.Context, teamID string) ([]models.DockerRegistry, error) {
	var regs []models.DockerRegistry

	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&regs).Error

	return regs, err
}

// Update saves changes to a docker registry
func (r *DockerRegistryRepository) Update(ctx context.Context, registry *models.DockerRegistry) error {
	return r.db.WithContext(ctx).Save(registry).Error
}

// Delete removes a docker registry by ID
func (r *DockerRegistryRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DockerRegistry{}, "id = ?", id).Error
}

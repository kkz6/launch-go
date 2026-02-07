package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerVolumeRepository handles database operations for docker service volumes
type DockerVolumeRepository struct {
	db *gorm.DB
}

// NewDockerVolumeRepository creates a new DockerVolumeRepository
func NewDockerVolumeRepository(db *gorm.DB) *DockerVolumeRepository {
	return &DockerVolumeRepository{db: db}
}

// Create inserts a new docker volume record
func (r *DockerVolumeRepository) Create(ctx context.Context, volume *models.DockerVolume) error {
	return r.db.WithContext(ctx).Create(volume).Error
}

// FindByServiceID retrieves all volumes for a docker service
func (r *DockerVolumeRepository) FindByServiceID(ctx context.Context, serviceID string) ([]models.DockerVolume, error) {
	var volumes []models.DockerVolume

	err := r.db.WithContext(ctx).
		Where("docker_service_id = ?", serviceID).
		Order("created_at ASC").
		Find(&volumes).Error

	return volumes, err
}

// Delete removes a docker volume by ID
func (r *DockerVolumeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DockerVolume{}, "id = ?", id).Error
}

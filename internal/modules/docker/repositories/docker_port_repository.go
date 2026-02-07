package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerPortRepository handles database operations for docker service ports
type DockerPortRepository struct {
	db *gorm.DB
}

// NewDockerPortRepository creates a new DockerPortRepository
func NewDockerPortRepository(db *gorm.DB) *DockerPortRepository {
	return &DockerPortRepository{db: db}
}

// Create inserts a new docker port record
func (r *DockerPortRepository) Create(ctx context.Context, port *models.DockerPort) error {
	return r.db.WithContext(ctx).Create(port).Error
}

// FindByServiceID retrieves all ports for a docker service
func (r *DockerPortRepository) FindByServiceID(ctx context.Context, serviceID string) ([]models.DockerPort, error) {
	var ports []models.DockerPort

	err := r.db.WithContext(ctx).
		Where("docker_service_id = ?", serviceID).
		Order("created_at ASC").
		Find(&ports).Error

	return ports, err
}

// Delete removes a docker port by ID
func (r *DockerPortRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DockerPort{}, "id = ?", id).Error
}

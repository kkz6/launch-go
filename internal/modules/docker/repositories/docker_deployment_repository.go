package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerDeploymentRepository handles database operations for docker deployments
type DockerDeploymentRepository struct {
	db *gorm.DB
}

// NewDockerDeploymentRepository creates a new DockerDeploymentRepository
func NewDockerDeploymentRepository(db *gorm.DB) *DockerDeploymentRepository {
	return &DockerDeploymentRepository{db: db}
}

// Create inserts a new docker deployment record
func (r *DockerDeploymentRepository) Create(ctx context.Context, deployment *models.DockerDeployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

// FindByID retrieves a docker deployment by ID
func (r *DockerDeploymentRepository) FindByID(ctx context.Context, id string) (*models.DockerDeployment, error) {
	var deployment models.DockerDeployment

	err := r.db.WithContext(ctx).
		First(&deployment, "id = ?", id).Error

	return &deployment, err
}

// FindByServiceID retrieves all deployments for a docker service
func (r *DockerDeploymentRepository) FindByServiceID(ctx context.Context, serviceID string) ([]models.DockerDeployment, error) {
	var deployments []models.DockerDeployment

	err := r.db.WithContext(ctx).
		Where("docker_service_id = ?", serviceID).
		Order("created_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// Update saves changes to a docker deployment
func (r *DockerDeploymentRepository) Update(ctx context.Context, deployment *models.DockerDeployment) error {
	return r.db.WithContext(ctx).Save(deployment).Error
}

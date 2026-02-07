package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerServiceRepository handles database operations for docker services
type DockerServiceRepository struct {
	db *gorm.DB
}

// NewDockerServiceRepository creates a new DockerServiceRepository
func NewDockerServiceRepository(db *gorm.DB) *DockerServiceRepository {
	return &DockerServiceRepository{db: db}
}

// Create inserts a new docker service record
func (r *DockerServiceRepository) Create(ctx context.Context, service *models.DockerService) error {
	return r.db.WithContext(ctx).Create(service).Error
}

// FindByID retrieves a docker service by ID with all relations
func (r *DockerServiceRepository) FindByID(ctx context.Context, id string) (*models.DockerService, error) {
	var svc models.DockerService

	err := r.db.WithContext(ctx).
		Preload("EnvVars").
		Preload("Volumes").
		Preload("Ports").
		Preload("Domains").
		First(&svc, "id = ?", id).Error

	return &svc, err
}

// FindByServerID retrieves all docker services for a server
func (r *DockerServiceRepository) FindByServerID(ctx context.Context, serverID string) ([]models.DockerService, error) {
	var svcs []models.DockerService

	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&svcs).Error

	return svcs, err
}

// Update saves changes to a docker service
func (r *DockerServiceRepository) Update(ctx context.Context, service *models.DockerService) error {
	return r.db.WithContext(ctx).Save(service).Error
}

// Delete removes a docker service by ID
func (r *DockerServiceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DockerService{}, "id = ?", id).Error
}

// UpdateStatus updates only the status field of a docker service
func (r *DockerServiceRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	return r.db.WithContext(ctx).
		Model(&models.DockerService{}).
		Where("id = ?", id).
		Update("status", status).Error
}

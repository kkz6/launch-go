package repositories

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerEnvVarRepository handles database operations for docker service env vars
type DockerEnvVarRepository struct {
	db *gorm.DB
}

// NewDockerEnvVarRepository creates a new DockerEnvVarRepository
func NewDockerEnvVarRepository(db *gorm.DB) *DockerEnvVarRepository {
	return &DockerEnvVarRepository{db: db}
}

// FindByServiceID retrieves all env vars for a docker service
func (r *DockerEnvVarRepository) FindByServiceID(ctx context.Context, serviceID string) ([]models.DockerEnvVar, error) {
	var envVars []models.DockerEnvVar

	err := r.db.WithContext(ctx).
		Where("docker_service_id = ?", serviceID).
		Order("created_at ASC").
		Find(&envVars).Error

	return envVars, err
}

// BulkReplace deletes all existing env vars for a service and inserts new ones in a transaction
func (r *DockerEnvVarRepository) BulkReplace(ctx context.Context, serviceID string, envVars []models.DockerEnvVar) ([]models.DockerEnvVar, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("docker_service_id = ?", serviceID).Delete(&models.DockerEnvVar{}).Error; err != nil {
			return fmt.Errorf("failed to delete existing env vars: %w", err)
		}

		if len(envVars) == 0 {
			return nil
		}

		for i := range envVars {
			envVars[i].DockerServiceID = serviceID
		}

		if err := tx.Create(&envVars).Error; err != nil {
			return fmt.Errorf("failed to create env vars: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return envVars, nil
}

package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/models"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DockerServiceRepository handles docker service database operations.
type DockerServiceRepository struct {
	repository.Base[models.DockerService]
}

// NewDockerServiceRepository creates a new docker service repository.
func NewDockerServiceRepository(db *gorm.DB) *DockerServiceRepository {
	return &DockerServiceRepository{
		Base: repository.NewBase[models.DockerService](db),
	}
}

// FindByServerAndKind returns the docker service for the given server and
// kind, or nil if none exists.
func (r *DockerServiceRepository) FindByServerAndKind(ctx context.Context, serverID string, kind types.Kind) (*models.DockerService, error) {
	var m models.DockerService
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND kind = ?", serverID, kind).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// FindByServer returns every docker service installed on a given server.
func (r *DockerServiceRepository) FindByServer(ctx context.Context, serverID string) ([]models.DockerService, error) {
	var services []models.DockerService
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at ASC").
		Find(&services).Error
	return services, err
}

// Update applies the given column updates to a docker service.
func (r *DockerServiceRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.DockerService{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiberutil.NotFound()
	}
	return nil
}

// Delete removes a docker service row by ID.
func (r *DockerServiceRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.DockerService{}, "id = ?", id).Error
}

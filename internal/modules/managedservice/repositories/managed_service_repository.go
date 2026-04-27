package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/managedservice/models"
	"github.com/kkz6/launch-go/internal/modules/managedservice/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ManagedServiceRepository handles managed service database operations.
type ManagedServiceRepository struct {
	repository.Base[models.ManagedService]
}

// NewManagedServiceRepository creates a new managed service repository.
func NewManagedServiceRepository(db *gorm.DB) *ManagedServiceRepository {
	return &ManagedServiceRepository{
		Base: repository.NewBase[models.ManagedService](db),
	}
}

// FindByServerAndKind returns the managed service for the given server and
// kind, or nil if none exists.
func (r *ManagedServiceRepository) FindByServerAndKind(ctx context.Context, serverID string, kind types.Kind) (*models.ManagedService, error) {
	var m models.ManagedService
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

// FindByServer returns every managed service installed on a given server.
func (r *ManagedServiceRepository) FindByServer(ctx context.Context, serverID string) ([]models.ManagedService, error) {
	var services []models.ManagedService
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at ASC").
		Find(&services).Error
	return services, err
}

// Update applies the given column updates to a managed service.
func (r *ManagedServiceRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.ManagedService{}).
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

// Delete removes a managed service row by ID.
func (r *ManagedServiceRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.ManagedService{}, "id = ?", id).Error
}

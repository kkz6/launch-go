package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ServiceRepository handles installed service database operations
type ServiceRepository struct {
	repository.Base[models.InstalledService]
}

// NewServiceRepository creates a new ServiceRepository instance
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{
		Base: repository.NewBase[models.InstalledService](db),
	}
}

// FindByID finds a service by ID
func (r *ServiceRepository) FindByID(ctx context.Context, id string) (*models.InstalledService, error) {
	service, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return service, nil
}

// FindByServerAndType finds all services by server and type
func (r *ServiceRepository) FindByServerAndType(ctx context.Context, serverID string, serviceType types.ServiceType) ([]models.InstalledService, error) {
	var services []models.InstalledService
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND type = ?", serverID, serviceType).
		Find(&services).Error
	return services, err
}

// FindOneByServerAndType finds a service by server and type
func (r *ServiceRepository) FindOneByServerAndType(ctx context.Context, serverID string, serviceType types.ServiceType) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND type = ?", serverID, serviceType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &service, nil
}

// FindByServerAndSoftware finds a service by server and software
func (r *ServiceRepository) FindByServerAndSoftware(ctx context.Context, serverID string, software types.Software) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND software = ?", serverID, software).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &service, nil
}

// FindDatabaseService finds the database service for a server
func (r *ServiceRepository) FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND (type = ? OR type = ?)", serverID, types.ServiceTypeMySQL, types.ServiceTypePostgreSQL).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &service, nil
}

// UpdateStatus updates the service status
func (r *ServiceRepository) UpdateStatus(ctx context.Context, id string, status types.ServiceStatus) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// UpdateWithTypeData updates the service status and type data
func (r *ServiceRepository) UpdateWithTypeData(ctx context.Context, id string, status types.ServiceStatus, typeData map[string]any) error {
	service, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if service.TypeData == nil {
		service.TypeData = make(map[string]any)
	}
	for k, v := range typeData {
		service.TypeData[k] = v
	}

	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"status":    status,
		"type_data": service.TypeData,
	})
}

// SetDefault sets the is_default flag for a service
func (r *ServiceRepository) SetDefault(ctx context.Context, id string, isDefault bool) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"is_default": isDefault,
	})
}

// UnsetDefaultPhp unsets the is_default flag for all PHP services on a server
func (r *ServiceRepository) UnsetDefaultPhp(ctx context.Context, serverID string) error {
	return r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("server_id = ? AND type = ?", serverID, types.ServiceTypePhp).
		Update("is_default", false).Error
}

// MarkRemovalFailed marks a service removal as failed
func (r *ServiceRepository) MarkRemovalFailed(ctx context.Context, id string) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"removal_requested_at": nil,
		"removal_failed_at":    time.Now(),
	})
}

// FindPhpByServerAndVersion finds a PHP service by server ID and version (e.g., "8.4")
func (r *ServiceRepository) FindPhpByServerAndVersion(ctx context.Context, serverID, version string) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND type = ? AND version = ?", serverID, types.ServiceTypePhp, version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &service, nil
}

// AddExtension adds an extension to a PHP service's type_data with "installed" status
func (r *ServiceRepository) AddExtension(ctx context.Context, id, extension string) error {
	return r.SetExtensionStatus(ctx, id, extension, "installed")
}

// SetExtensionStatus sets the status of an extension in a PHP service's type_data
func (r *ServiceRepository) SetExtensionStatus(ctx context.Context, id, extension, status string) error {
	service, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if service.TypeData == nil {
		service.TypeData = make(map[string]any)
	}

	extensions, ok := service.TypeData["extensions"].(map[string]any)
	if !ok {
		extensions = make(map[string]any)
	}

	extensions[extension] = map[string]any{
		"status": status,
	}
	service.TypeData["extensions"] = extensions

	return r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Update("type_data", service.TypeData).Error
}

// RemoveExtension removes an extension from a PHP service's type_data
func (r *ServiceRepository) RemoveExtension(ctx context.Context, id, extension string) error {
	service, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if service.TypeData == nil {
		return nil
	}

	extensions, ok := service.TypeData["extensions"].(map[string]any)
	if !ok {
		return nil
	}

	delete(extensions, extension)
	service.TypeData["extensions"] = extensions

	return r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Update("type_data", service.TypeData).Error
}

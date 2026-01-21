package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
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
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return service, nil
}

// FindByServer finds all services for a server
func (r *ServiceRepository) FindByServer(ctx context.Context, serverID string) ([]models.InstalledService, error) {
	return r.Base.FindByServer(ctx, serverID)
}

// FindByServerAndType finds all services by server and type
func (r *ServiceRepository) FindByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) ([]models.InstalledService, error) {
	var services []models.InstalledService
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND type = ?", serverID, serviceType).
		Find(&services).Error
	return services, err
}

// FindOneByServerAndType finds a service by server and type
func (r *ServiceRepository) FindOneByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND type = ?", serverID, serviceType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindByServerAndSoftware finds a service by server and software
func (r *ServiceRepository) FindByServerAndSoftware(ctx context.Context, serverID string, software enums.Software) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND software = ?", serverID, software).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindDatabaseService finds the database service for a server
func (r *ServiceRepository) FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB.WithContext(ctx).
		First(&service, "server_id = ? AND (type = ? OR type = ?)", serverID, enums.ServiceTypeMySql, enums.ServiceTypePostgreSql).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// UpdateStatus updates the service status
func (r *ServiceRepository) UpdateStatus(ctx context.Context, id string, status enums.ServiceStatus) error {
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// UpdateWithTypeData updates the service status and type data
func (r *ServiceRepository) UpdateWithTypeData(ctx context.Context, id string, status enums.ServiceStatus, typeData map[string]any) error {
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
		Where("server_id = ? AND type = ?", serverID, enums.ServiceTypePhp).
		Update("is_default", false).Error
}

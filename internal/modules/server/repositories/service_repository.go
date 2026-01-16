package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServiceRepository handles installed service database operations
type ServiceRepository struct {
	BaseRepository
}

// NewServiceRepository creates a new ServiceRepository instance
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new service
func (r *ServiceRepository) Create(ctx context.Context, service *models.InstalledService) error {
	return r.DB().WithContext(ctx).Create(service).Error
}

// FindByID finds a service by ID
func (r *ServiceRepository) FindByID(ctx context.Context, id string) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB().WithContext(ctx).First(&service, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindByServer finds all services for a server
func (r *ServiceRepository) FindByServer(ctx context.Context, serverID string) ([]models.InstalledService, error) {
	var services []models.InstalledService
	err := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&services).Error
	return services, err
}

// FindByServerAndType finds all services by server and type
func (r *ServiceRepository) FindByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) ([]models.InstalledService, error) {
	var services []models.InstalledService
	err := r.DB().WithContext(ctx).
		Where("server_id = ? AND type = ?", serverID, serviceType).
		Find(&services).Error
	return services, err
}

// FindOneByServerAndType finds a service by server and type
func (r *ServiceRepository) FindOneByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.DB().WithContext(ctx).
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
	err := r.DB().WithContext(ctx).
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
	err := r.DB().WithContext(ctx).
		First(&service, "server_id = ? AND (type = ? OR type = ?)", serverID, enums.ServiceTypeMySql, enums.ServiceTypePostgreSql).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// Update updates a service
func (r *ServiceRepository) Update(ctx context.Context, service *models.InstalledService) error {
	return r.DB().WithContext(ctx).Save(service).Error
}

// UpdateStatus updates the service status
func (r *ServiceRepository) UpdateStatus(ctx context.Context, id string, status enums.ServiceStatus) error {
	return r.DB().WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Update("status", status).Error
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

	return r.DB().WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":    status,
			"type_data": service.TypeData,
		}).Error
}

// Delete deletes a service
func (r *ServiceRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.InstalledService{}, "id = ?", id).Error
}

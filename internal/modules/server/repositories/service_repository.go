package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateService creates a new service
func (r *Repository) CreateService(ctx context.Context, service *models.InstalledService) error {
	return create(r, ctx, service)
}

// FindServiceByID finds a service by ID
func (r *Repository) FindServiceByID(ctx context.Context, id string) (*models.InstalledService, error) {
	return findByID[models.InstalledService](r, ctx, id, ErrServiceNotFound)
}

// FindServicesByServer finds all services for a server
func (r *Repository) FindServicesByServer(ctx context.Context, serverID string) ([]models.InstalledService, error) {
	return findByServer[models.InstalledService](r, ctx, serverID)
}

// FindServicesByServerAndType finds all services by server and type
func (r *Repository) FindServicesByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) ([]models.InstalledService, error) {
	var services []models.InstalledService
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND type = ?", serverID, serviceType).
		Find(&services).Error
	if err != nil {
		return nil, err
	}
	return services, nil
}

// FindServiceByServerAndType finds a service by server and type
func (r *Repository) FindServiceByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.db.WithContext(ctx).
		First(&service, "server_id = ? AND type = ?", serverID, serviceType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindServiceByServerAndSoftware finds a service by server and software
func (r *Repository) FindServiceByServerAndSoftware(ctx context.Context, serverID string, software enums.Software) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.db.WithContext(ctx).
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
func (r *Repository) FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error) {
	var service models.InstalledService
	err := r.db.WithContext(ctx).
		First(&service, "server_id = ? AND (type = ? OR type = ?)", serverID, enums.ServiceTypeMySql, enums.ServiceTypePostgreSql).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// UpdateService updates a service
func (r *Repository) UpdateService(ctx context.Context, service *models.InstalledService) error {
	return update(r, ctx, service)
}

// UpdateServiceStatus updates the service status
func (r *Repository) UpdateServiceStatus(ctx context.Context, id string, status enums.ServiceStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateServiceWithTypeData updates the service status and type data
func (r *Repository) UpdateServiceWithTypeData(ctx context.Context, id string, status enums.ServiceStatus, typeData map[string]any) error {
	service, err := r.FindServiceByID(ctx, id)
	if err != nil {
		return err
	}

	// Merge existing type_data with new data
	if service.TypeData == nil {
		service.TypeData = make(map[string]any)
	}
	for k, v := range typeData {
		service.TypeData[k] = v
	}

	return r.db.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":    status,
			"type_data": service.TypeData,
		}).Error
}

// DeleteService deletes a service
func (r *Repository) DeleteService(ctx context.Context, id string) error {
	return deleteByID[models.InstalledService](r, ctx, id)
}

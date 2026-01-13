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

// DeleteService deletes a service
func (r *Repository) DeleteService(ctx context.Context, id string) error {
	return deleteByID[models.InstalledService](r, ctx, id)
}

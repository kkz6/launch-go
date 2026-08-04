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
	return repository.FindAll[models.InstalledService](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.WithType(string(serviceType)),
	)
}

// FindOneByServerAndType finds a service by server and type
func (r *ServiceRepository) FindOneByServerAndType(ctx context.Context, serverID string, serviceType types.ServiceType) (*models.InstalledService, error) {
	return repository.FindOne[models.InstalledService](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.WithType(string(serviceType)),
	)
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
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// ClaimPhpPatch atomically reserves the active state observed by the caller.
// Requiring that exact prior state prevents a second lifecycle operation from
// taking ownership of an existing patch reservation.
func (r *ServiceRepository) ClaimPhpPatch(
	ctx context.Context,
	id string,
	previousStatus types.ServiceStatus,
) (bool, error) {
	if !previousStatus.IsActive() {
		return false, nil
	}

	result := r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ? AND type = ? AND status = ?", id, types.ServiceTypePhp, previousStatus).
		Update("status", types.ServiceStatusUpdating)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

// RestorePhpPatchStatus releases a patch reservation back to its exact prior
// active state. The conditional update cannot overwrite a completed patch.
func (r *ServiceRepository) RestorePhpPatchStatus(
	ctx context.Context,
	id string,
	status types.ServiceStatus,
) (bool, error) {
	if !status.IsActive() {
		return false, nil
	}

	result := r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ? AND status = ?", id, types.ServiceStatusUpdating).
		Update("status", status)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

// UpdateStatusFromProbe merges diagnostic output while protecting lifecycle
// states. The final conditional write closes the race where a patch is claimed
// after the probe reads the service but before it persists the result.
func (r *ServiceRepository) UpdateStatusFromProbe(
	ctx context.Context,
	id string,
	status types.ServiceStatus,
	typeData map[string]any,
) (bool, error) {
	service, err := r.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if service.Status == types.ServiceStatusUpdating {
		return false, nil
	}

	if service.TypeData == nil {
		service.TypeData = make(map[string]any)
	}
	for key, value := range typeData {
		service.TypeData[key] = value
	}

	result := r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("id = ? AND status <> ?", id, types.ServiceStatusUpdating).
		Updates(map[string]any{
			"status":    status,
			"type_data": service.TypeData,
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
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

	return r.UpdateFields(ctx, id, map[string]interface{}{
		"status":    status,
		"type_data": service.TypeData,
	})
}

// SetDefault sets the is_default flag for a service. When marking a
// service as default, all sibling services of the same type on the
// same server are atomically unset — there can only be one default
// per (server_id, type). Belt-and-braces against callers forgetting
// to call UnsetDefaultPhp first (which is how the two-stars-on-PHP
// bug crept in: schema column default was TRUE, so newly-installed
// PHP versions auto-claimed the flag without ever going through
// SetDefault).
func (r *ServiceRepository) SetDefault(ctx context.Context, id string, isDefault bool) error {
	if !isDefault {
		// Clearing a single row is harmless — no exclusion required.
		return r.UpdateFields(ctx, id, map[string]interface{}{
			"is_default": false,
		})
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Resolve the target's (server_id, type) so the unset-siblings
		// update can be scoped without a second round-trip.
		var target models.InstalledService
		if err := tx.Select("id", "server_id", "type").
			First(&target, "id = ?", id).Error; err != nil {
			return err
		}
		// Unset every other row in the same (server_id, type) group
		// before flipping the target on. Doing it in this order avoids
		// the brief window where two rows could appear default.
		if err := tx.Model(&models.InstalledService{}).
			Where("server_id = ? AND type = ? AND id <> ?", target.ServerID, target.Type, id).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&models.InstalledService{}).
			Where("id = ?", id).
			Update("is_default", true).Error
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
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"removal_requested_at": nil,
		"removal_failed_at":    time.Now(),
	})
}

// FindPhpByServerAndVersion finds a PHP service by server ID and version (e.g., "8.4")
func (r *ServiceRepository) FindPhpByServerAndVersion(ctx context.Context, serverID, version string) (*models.InstalledService, error) {
	var service models.InstalledService
	query := r.DB.WithContext(ctx).
		Where("server_id = ? AND type = ?", serverID, types.ServiceTypePhp)

	software := types.SoftwareFromPhpVersion(version)
	if software.IsPhp() {
		series := software.GetVersion()
		query = query.Where("software = ? OR version = ? OR version LIKE ?", software, series, series+".%")
	} else {
		query = query.Where("version = ?", version)
	}

	err := query.First(&service).Error
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

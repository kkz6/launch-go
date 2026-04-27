package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// EnvVarRepository handles docker-app env-var database operations.
type EnvVarRepository struct {
	repository.Base[models.EnvVar]
}

// NewEnvVarRepository constructs the repository.
func NewEnvVarRepository(db *gorm.DB) *EnvVarRepository {
	return &EnvVarRepository{Base: repository.NewBase[models.EnvVar](db)}
}

// FindByApp returns env vars for an app, ordered alphabetically by key.
func (r *EnvVarRepository) FindByApp(ctx context.Context, appID string) ([]models.EnvVar, error) {
	var out []models.EnvVar
	err := r.DB.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("`key` ASC").
		Find(&out).Error
	return out, err
}

// Update applies partial updates to an env var row.
func (r *EnvVarRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.EnvVar{}).
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

// Delete removes an env var row.
func (r *EnvVarRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.EnvVar{}, "id = ?", id).Error
}

// DeleteByApp removes every env var attached to an app.
func (r *EnvVarRepository) DeleteByApp(ctx context.Context, appID string) error {
	return r.DB.WithContext(ctx).Where("app_id = ?", appID).Delete(&models.EnvVar{}).Error
}

// PortRepository handles docker-app port database operations.
type PortRepository struct {
	repository.Base[models.Port]
}

// NewPortRepository constructs the repository.
func NewPortRepository(db *gorm.DB) *PortRepository {
	return &PortRepository{Base: repository.NewBase[models.Port](db)}
}

// FindByApp returns ports for an app, ordered by host port.
func (r *PortRepository) FindByApp(ctx context.Context, appID string) ([]models.Port, error) {
	var out []models.Port
	err := r.DB.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("host_port ASC").
		Find(&out).Error
	return out, err
}

// Update applies partial updates to a port row.
func (r *PortRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.Port{}).
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

// Delete removes a port row.
func (r *PortRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Port{}, "id = ?", id).Error
}

// VolumeRepository handles docker-app volume database operations.
type VolumeRepository struct {
	repository.Base[models.Volume]
}

// NewVolumeRepository constructs the repository.
func NewVolumeRepository(db *gorm.DB) *VolumeRepository {
	return &VolumeRepository{Base: repository.NewBase[models.Volume](db)}
}

// FindByApp returns volumes for an app, ordered by name.
func (r *VolumeRepository) FindByApp(ctx context.Context, appID string) ([]models.Volume, error) {
	var out []models.Volume
	err := r.DB.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("name ASC").
		Find(&out).Error
	return out, err
}

// Delete removes a volume row.
func (r *VolumeRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Volume{}, "id = ?", id).Error
}

// DomainRepository handles docker-app domain database operations.
type DomainRepository struct {
	repository.Base[models.Domain]
}

// NewDomainRepository constructs the repository.
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{Base: repository.NewBase[models.Domain](db)}
}

// FindByApp returns domains for an app, ordered alphabetically.
func (r *DomainRepository) FindByApp(ctx context.Context, appID string) ([]models.Domain, error) {
	var out []models.Domain
	err := r.DB.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("domain ASC").
		Find(&out).Error
	return out, err
}

// FindByDomain returns the row matching a domain string, or nil.
func (r *DomainRepository) FindByDomain(ctx context.Context, domain string) (*models.Domain, error) {
	var d models.Domain
	err := r.DB.WithContext(ctx).
		Where("domain = ?", domain).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

// Update applies partial updates to a domain row.
func (r *DomainRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.Domain{}).
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

// Delete removes a domain row.
func (r *DomainRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.Domain{}, "id = ?", id).Error
}

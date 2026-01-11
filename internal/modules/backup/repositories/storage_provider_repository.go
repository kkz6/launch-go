package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

// StorageProviderRepository handles database operations for storage providers
type StorageProviderRepository struct {
	db *gorm.DB
}

// NewStorageProviderRepository creates a new storage provider repository
func NewStorageProviderRepository(db *gorm.DB) *StorageProviderRepository {
	return &StorageProviderRepository{db: db}
}

// CreateStorageProvider creates a new storage provider
func (r *StorageProviderRepository) CreateStorageProvider(ctx context.Context, provider *models.StorageProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// FindStorageProviderByID finds a storage provider by ID
func (r *StorageProviderRepository) FindStorageProviderByID(ctx context.Context, id uint64) (*models.StorageProvider, error) {
	var provider models.StorageProvider
	err := r.db.WithContext(ctx).First(&provider, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrStorageProviderNotFound
	}

	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// FindStorageProviderByIDString finds a storage provider by string ID
func (r *StorageProviderRepository) FindStorageProviderByIDString(ctx context.Context, id string) (*models.StorageProvider, error) {
	var provider models.StorageProvider
	err := r.db.WithContext(ctx).First(&provider, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrStorageProviderNotFound
	}

	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// FindStorageProvidersByTeamID finds all storage providers for a team
func (r *StorageProviderRepository) FindStorageProvidersByTeamID(ctx context.Context, teamID string) ([]models.StorageProvider, error) {
	var providers []models.StorageProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error

	return providers, err
}

// FindStorageProvidersByDriver finds all storage providers of a specific type
func (r *StorageProviderRepository) FindStorageProvidersByDriver(ctx context.Context, driver enums.StorageDriver) ([]models.StorageProvider, error) {
	var providers []models.StorageProvider
	err := r.db.WithContext(ctx).
		Where("provider = ?", driver).
		Find(&providers).Error

	return providers, err
}

// FindStorageProvidersByTeamAndDriver finds storage providers by team and driver
func (r *StorageProviderRepository) FindStorageProvidersByTeamAndDriver(ctx context.Context, teamID string, driver enums.StorageDriver) ([]models.StorageProvider, error) {
	var providers []models.StorageProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND provider = ?", teamID, driver).
		Find(&providers).Error

	return providers, err
}

// UpdateStorageProvider updates a storage provider
func (r *StorageProviderRepository) UpdateStorageProvider(ctx context.Context, provider *models.StorageProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// DeleteStorageProvider deletes a storage provider
func (r *StorageProviderRepository) DeleteStorageProvider(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&models.StorageProvider{}, id).Error
}

// StorageProviderExists checks if a storage provider exists
func (r *StorageProviderRepository) StorageProviderExists(ctx context.Context, id uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.StorageProvider{}).
		Where("id = ?", id).
		Count(&count).Error

	return count > 0, err
}

// HasBackupsForStorageProvider checks if a storage provider has any associated backups
func (r *StorageProviderRepository) HasBackupsForStorageProvider(ctx context.Context, providerID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Backup{}).
		Where("storage_provider_id = ?", providerID).
		Count(&count).Error

	return count > 0, err
}

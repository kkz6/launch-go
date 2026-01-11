package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

// BackupRepository handles database operations for backups
type BackupRepository struct {
	db *gorm.DB
}

// NewBackupRepository creates a new backup repository
func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// CreateBackup creates a new backup configuration
func (r *BackupRepository) CreateBackup(ctx context.Context, backup *models.Backup) error {
	return r.db.WithContext(ctx).Create(backup).Error
}

// CreateBackupWithDatabases creates a backup and associates it with databases
func (r *BackupRepository) CreateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(backup).Error; err != nil {
			return err
		}

		for _, dbID := range databaseIDs {
			backupDB := &models.BackupDatabase{
				BackupID:   backup.ID,
				DatabaseID: dbID,
			}
			if err := tx.Create(backupDB).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// FindBackupByID finds a backup by ID
func (r *BackupRepository) FindBackupByID(ctx context.Context, id string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.WithContext(ctx).
		Preload("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}).
		Preload("StorageProvider").
		Preload("Databases").
		First(&backup, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBackupNotFound
	}

	if err != nil {
		return nil, err
	}

	return &backup, nil
}

// FindBackupByIDAndServer finds a backup by ID and server ID
func (r *BackupRepository) FindBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.WithContext(ctx).
		Preload("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}).
		Preload("StorageProvider").
		Preload("Databases").
		First(&backup, "id = ? AND server_id = ?", id, serverID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBackupNotFound
	}

	if err != nil {
		return nil, err
	}

	return &backup, nil
}

// FindBackupsByServerID finds all backups for a server
func (r *BackupRepository) FindBackupsByServerID(ctx context.Context, serverID string) ([]models.Backup, error) {
	var backups []models.Backup
	err := r.db.WithContext(ctx).
		Preload("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}).
		Preload("StorageProvider").
		Preload("Databases").
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&backups).Error

	return backups, err
}

// UpdateBackup updates a backup configuration
func (r *BackupRepository) UpdateBackup(ctx context.Context, backup *models.Backup) error {
	return r.db.WithContext(ctx).Save(backup).Error
}

// UpdateBackupWithDatabases updates a backup and its associated databases
func (r *BackupRepository) UpdateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(backup).Error; err != nil {
			return err
		}

		// Delete existing associations
		if err := tx.Where("backup_id = ?", backup.ID).Delete(&models.BackupDatabase{}).Error; err != nil {
			return err
		}

		// Create new associations
		for _, dbID := range databaseIDs {
			backupDB := &models.BackupDatabase{
				BackupID:   backup.ID,
				DatabaseID: dbID,
			}
			if err := tx.Create(backupDB).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateBackupFields updates specific fields of a backup
func (r *BackupRepository) UpdateBackupFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Backup{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// DeleteBackup soft deletes a backup
func (r *BackupRepository) DeleteBackup(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Backup{}, "id = ?", id).Error
}

// GetLatestBackupByServerID gets the most recent backup for a server
func (r *BackupRepository) GetLatestBackupByServerID(ctx context.Context, serverID string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		First(&backup).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &backup, nil
}

// BackupExists checks if a backup exists
func (r *BackupRepository) BackupExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Backup{}).
		Where("id = ?", id).
		Count(&count).Error

	return count > 0, err
}

// SyncBackupDatabases syncs the databases for a backup
func (r *BackupRepository) SyncBackupDatabases(ctx context.Context, backupID string, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing associations
		if err := tx.Where("backup_id = ?", backupID).Delete(&models.BackupDatabase{}).Error; err != nil {
			return err
		}

		// Create new associations
		for _, dbID := range databaseIDs {
			backupDB := &models.BackupDatabase{
				BackupID:   backupID,
				DatabaseID: dbID,
			}
			if err := tx.Create(backupDB).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetBackupDatabaseIDs gets the database IDs associated with a backup
func (r *BackupRepository) GetBackupDatabaseIDs(ctx context.Context, backupID string) ([]string, error) {
	var backupDBs []models.BackupDatabase
	err := r.db.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Find(&backupDBs).Error
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(backupDBs))
	for i, bd := range backupDBs {
		ids[i] = bd.DatabaseID
	}

	return ids, nil
}

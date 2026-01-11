package backup

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrBackupNotFound          = errors.New("backup not found")
	ErrStorageProviderNotFound = errors.New("storage provider not found")
	ErrBackupJobNotFound       = errors.New("backup job not found")
)

// Repository handles database operations for the backup module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new backup repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Backup operations

// CreateBackup creates a new backup configuration
func (r *Repository) CreateBackup(ctx context.Context, backup *Backup) error {
	return r.db.WithContext(ctx).Create(backup).Error
}

// CreateBackupWithDatabases creates a backup and associates it with databases
func (r *Repository) CreateBackupWithDatabases(ctx context.Context, backup *Backup, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(backup).Error; err != nil {
			return err
		}

		for _, dbID := range databaseIDs {
			backupDB := &BackupDatabase{
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
func (r *Repository) FindBackupByID(ctx context.Context, id string) (*Backup, error) {
	var backup Backup
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
func (r *Repository) FindBackupByIDAndServer(ctx context.Context, id, serverID string) (*Backup, error) {
	var backup Backup
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
func (r *Repository) FindBackupsByServerID(ctx context.Context, serverID string) ([]Backup, error) {
	var backups []Backup
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
func (r *Repository) UpdateBackup(ctx context.Context, backup *Backup) error {
	return r.db.WithContext(ctx).Save(backup).Error
}

// UpdateBackupWithDatabases updates a backup and its associated databases
func (r *Repository) UpdateBackupWithDatabases(ctx context.Context, backup *Backup, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(backup).Error; err != nil {
			return err
		}

		// Delete existing associations
		if err := tx.Where("backup_id = ?", backup.ID).Delete(&BackupDatabase{}).Error; err != nil {
			return err
		}

		// Create new associations
		for _, dbID := range databaseIDs {
			backupDB := &BackupDatabase{
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
func (r *Repository) UpdateBackupFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Backup{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// DeleteBackup soft deletes a backup
func (r *Repository) DeleteBackup(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Backup{}, "id = ?", id).Error
}

// GetLatestBackupByServerID gets the most recent backup for a server
func (r *Repository) GetLatestBackupByServerID(ctx context.Context, serverID string) (*Backup, error) {
	var backup Backup
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

// BackupJob operations

// CreateBackupJob creates a new backup job
func (r *Repository) CreateBackupJob(ctx context.Context, job *BackupJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

// FindBackupJobByID finds a backup job by ID
func (r *Repository) FindBackupJobByID(ctx context.Context, id string) (*BackupJob, error) {
	var job BackupJob
	err := r.db.WithContext(ctx).
		Preload("Backup").
		Preload("StorageProvider").
		First(&job, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBackupJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// FindBackupJobsByBackupID finds all jobs for a backup
func (r *Repository) FindBackupJobsByBackupID(ctx context.Context, backupID string) ([]BackupJob, error) {
	var jobs []BackupJob
	err := r.db.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

// FindFinishedBackupJobs finds finished jobs for a backup
func (r *Repository) FindFinishedBackupJobs(ctx context.Context, backupID string) ([]BackupJob, error) {
	var jobs []BackupJob
	err := r.db.WithContext(ctx).
		Where("backup_id = ? AND status = ?", backupID, BackupJobStatusFinished).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

// UpdateBackupJob updates a backup job
func (r *Repository) UpdateBackupJob(ctx context.Context, job *BackupJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

// DeleteBackupJob deletes a backup job
func (r *Repository) DeleteBackupJob(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&BackupJob{}, "id = ?", id).Error
}

// GetBackupJobsTotalSize gets the total size of all finished jobs for a backup
func (r *Repository) GetBackupJobsTotalSize(ctx context.Context, backupID string) (int64, error) {
	var totalSize int64
	err := r.db.WithContext(ctx).
		Model(&BackupJob{}).
		Where("backup_id = ? AND status = ?", backupID, BackupJobStatusFinished).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalSize).Error
	return totalSize, err
}

// StorageProvider operations

// CreateStorageProvider creates a new storage provider
func (r *Repository) CreateStorageProvider(ctx context.Context, provider *StorageProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// FindStorageProviderByID finds a storage provider by ID
func (r *Repository) FindStorageProviderByID(ctx context.Context, id uint) (*StorageProvider, error) {
	var provider StorageProvider
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
func (r *Repository) FindStorageProviderByIDString(ctx context.Context, id string) (*StorageProvider, error) {
	var provider StorageProvider
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
func (r *Repository) FindStorageProvidersByTeamID(ctx context.Context, teamID string) ([]StorageProvider, error) {
	var providers []StorageProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	return providers, err
}

// FindStorageProvidersByDriver finds all storage providers of a specific type
func (r *Repository) FindStorageProvidersByDriver(ctx context.Context, driver StorageDriver) ([]StorageProvider, error) {
	var providers []StorageProvider
	err := r.db.WithContext(ctx).
		Where("provider = ?", driver).
		Find(&providers).Error
	return providers, err
}

// FindStorageProvidersByTeamAndDriver finds storage providers by team and driver
func (r *Repository) FindStorageProvidersByTeamAndDriver(ctx context.Context, teamID string, driver StorageDriver) ([]StorageProvider, error) {
	var providers []StorageProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND provider = ?", teamID, driver).
		Find(&providers).Error
	return providers, err
}

// UpdateStorageProvider updates a storage provider
func (r *Repository) UpdateStorageProvider(ctx context.Context, provider *StorageProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// DeleteStorageProvider deletes a storage provider
func (r *Repository) DeleteStorageProvider(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&StorageProvider{}, id).Error
}

// BackupDatabase operations

// SyncBackupDatabases syncs the databases for a backup
func (r *Repository) SyncBackupDatabases(ctx context.Context, backupID string, databaseIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing associations
		if err := tx.Where("backup_id = ?", backupID).Delete(&BackupDatabase{}).Error; err != nil {
			return err
		}

		// Create new associations
		for _, dbID := range databaseIDs {
			backupDB := &BackupDatabase{
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
func (r *Repository) GetBackupDatabaseIDs(ctx context.Context, backupID string) ([]string, error) {
	var backupDBs []BackupDatabase
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

// Exists methods

// BackupExists checks if a backup exists
func (r *Repository) BackupExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Backup{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// StorageProviderExists checks if a storage provider exists
func (r *Repository) StorageProviderExists(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&StorageProvider{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// HasBackupsForStorageProvider checks if a storage provider has any associated backups
func (r *Repository) HasBackupsForStorageProvider(ctx context.Context, providerID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Backup{}).
		Where("storage_provider_id = ?", providerID).
		Count(&count).Error
	return count > 0, err
}

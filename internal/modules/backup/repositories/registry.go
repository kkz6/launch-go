package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all backup module repositories
type Registry struct {
	backup          *BackupRepository
	backupJob       *BackupJobRepository
	storageProvider *StorageProviderRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		backup:          NewBackupRepository(db),
		backupJob:       NewBackupJobRepository(db),
		storageProvider: NewStorageProviderRepository(db),
	}
}

// Backup returns the backup repository
func (r *Registry) Backup() *BackupRepository { return r.backup }

// BackupJob returns the backup job repository
func (r *Registry) BackupJob() *BackupJobRepository { return r.backupJob }

// StorageProvider returns the storage provider repository
func (r *Registry) StorageProvider() *StorageProviderRepository { return r.storageProvider }

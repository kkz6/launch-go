package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
)

// BackupRepository defines the interface for backup database operations
type BackupRepository interface {
	// Backup operations
	CreateBackup(ctx context.Context, backup *models.Backup) error
	CreateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error
	FindBackupByID(ctx context.Context, id string) (*models.Backup, error)
	FindBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error)
	FindBackupByIDAndServerAndTeam(ctx context.Context, id, serverID, teamID string) (*models.Backup, error)
	FindBackupsByServerID(ctx context.Context, serverID string) ([]models.Backup, error)
	FindBackupsByServerAndTeam(ctx context.Context, serverID, teamID string) ([]models.Backup, error)
	UpdateBackup(ctx context.Context, backup *models.Backup) error
	UpdateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error
	UpdateBackupFields(ctx context.Context, id string, fields map[string]interface{}) error
	DeleteBackup(ctx context.Context, id string) error
	GetLatestBackupByServerID(ctx context.Context, serverID string) (*models.Backup, error)
	BackupExists(ctx context.Context, id string) (bool, error)

	// BackupDatabase operations
	SyncBackupDatabases(ctx context.Context, backupID string, databaseIDs []string) error
	GetBackupDatabaseIDs(ctx context.Context, backupID string) ([]string, error)
}

// BackupJobRepository defines the interface for backup job database operations
type BackupJobRepository interface {
	CreateBackupJob(ctx context.Context, job *models.BackupJob) error
	FindBackupJobByID(ctx context.Context, id string) (*models.BackupJob, error)
	FindBackupJobForRun(ctx context.Context, id, backupID, teamID string) (*models.BackupJob, error)
	ClaimPendingBackupJobForRun(ctx context.Context, id, backupID, teamID string, allowRunning bool) (*models.BackupJob, bool, error)
	FindBackupJobsByBackupID(ctx context.Context, backupID string) ([]models.BackupJob, error)
	FindFinishedBackupJobs(ctx context.Context, backupID string) ([]models.BackupJob, error)
	UpdateBackupJob(ctx context.Context, job *models.BackupJob) error
	MarkBackupJobRunningForRun(ctx context.Context, id, backupID, teamID, taskID string) (bool, error)
	MarkBackupJobFinishedForRun(ctx context.Context, id, backupID, teamID string, size *int, taskID *string) (bool, error)
	MarkBackupJobFailedForRun(ctx context.Context, id, backupID, teamID, message string, taskID *string) (bool, error)
	DeleteBackupJob(ctx context.Context, id string) error
	GetBackupJobsTotalSize(ctx context.Context, backupID string) (int64, error)
}

// StorageProviderRepository defines the interface for storage provider database operations
type StorageProviderRepository interface {
	CreateStorageProvider(ctx context.Context, provider *models.StorageProvider) error
	FindStorageProviderByID(ctx context.Context, id uint64) (*models.StorageProvider, error)
	FindStorageProviderByIDAndTeam(ctx context.Context, id uint64, teamID string) (*models.StorageProvider, error)
	FindStorageProviderDriverByIDAndTeam(ctx context.Context, id uint64, teamID string) (backuptypes.StorageDriver, error)
	FindStorageProviderByIDString(ctx context.Context, id string) (*models.StorageProvider, error)
	FindStorageProvidersByTeamID(ctx context.Context, teamID string) ([]models.StorageProvider, error)
	FindStorageProvidersByDriver(ctx context.Context, driver backuptypes.StorageDriver) ([]models.StorageProvider, error)
	FindStorageProvidersByTeamAndDriver(ctx context.Context, teamID string, driver backuptypes.StorageDriver) ([]models.StorageProvider, error)
	UpdateStorageProvider(ctx context.Context, provider *models.StorageProvider) error
	DeleteStorageProvider(ctx context.Context, id uint64) error
	StorageProviderExists(ctx context.Context, id uint64) (bool, error)
	HasBackupsForStorageProvider(ctx context.Context, providerID uint64) (bool, error)
}

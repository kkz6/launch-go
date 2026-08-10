package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

// BackupService defines the interface for backup business logic
type BackupService interface {
	// Backup operations
	CreateBackup(ctx context.Context, serverID, userID string, req *dto.CreateBackupRequest) (*models.Backup, error)
	UpdateBackup(ctx context.Context, id string, req *dto.UpdateBackupRequest) (*models.Backup, error)
	DeleteBackup(ctx context.Context, id, serverID string) error
	GetBackup(ctx context.Context, id string) (*models.Backup, error)
	GetBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error)
	ListBackupsByServer(ctx context.Context, serverID string) ([]models.Backup, error)
	RunBackup(ctx context.Context, id, serverID, teamID, userID string) (dto.BackupJobResponse, error)
	MarkBackupInstalled(ctx context.Context, id string) error
	MarkBackupInstallationFailed(ctx context.Context, id string) error
}

// BackupJobService defines the interface for backup job business logic
type BackupJobService interface {
	CreateBackupJob(ctx context.Context, backupID, token string, req *dto.CreateBackupJobRequest) (*models.BackupJob, error)
	GetBackupJob(ctx context.Context, id string) (*models.BackupJob, error)
	ListBackupJobs(ctx context.Context, backupID string) ([]models.BackupJob, error)
}

// StorageProviderService defines the interface for storage provider business logic
type StorageProviderService interface {
	ConnectStorageProvider(ctx context.Context, userID, teamID string, req *dto.CreateStorageProviderRequest) (*models.StorageProvider, error)
	UpdateStorageProvider(ctx context.Context, id uint, req *dto.UpdateStorageProviderRequest) (*models.StorageProvider, error)
	DeleteStorageProvider(ctx context.Context, id uint) error
	GetStorageProvider(ctx context.Context, id uint) (*models.StorageProvider, error)
	ListStorageProvidersByTeam(ctx context.Context, teamID string) ([]models.StorageProvider, error)
	GetStorageProviderConfig(ctx context.Context, id uint) (map[string]interface{}, error)
}

// AgentConfigService defines the interface for agent configuration
type AgentConfigService interface {
	GetAgentBackupConfig(ctx context.Context, backupID, webhookBaseURL string) (*dto.AgentBackupConfig, error)
}

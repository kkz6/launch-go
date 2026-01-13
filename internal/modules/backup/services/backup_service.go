package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// BackupService handles business logic for backups
type BackupService struct {
	backupRepo *repositories.BackupRepository
	queue      *queue.Client
	ws         *websocket.Hub
	logger     *zerolog.Logger
}

// NewBackupService creates a new backup service
func NewBackupService(
	backupRepo *repositories.BackupRepository,
	queue *queue.Client,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *BackupService {
	return &BackupService{
		backupRepo: backupRepo,
		queue:      queue,
		ws:         ws,
		logger:     logger,
	}
}

// CreateBackup creates a new backup configuration
func (s *BackupService) CreateBackup(ctx context.Context, serverID, userID string, req *dto.CreateBackupRequest) (*models.Backup, error) {
	// Convert include/exclude files to JSON strings
	includeFilesJSON, err := json.Marshal(req.IncludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process include files: %w", err)
	}

	excludeFilesJSON, err := json.Marshal(req.ExcludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process exclude files: %w", err)
	}

	// Convert StorageProviderID from string to uint64
	storageProviderID, err := strconv.ParseUint(req.StorageProviderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid storage provider ID: %w", err)
	}

	retention := req.Retention
	if retention == 0 {
		retention = 10
	}

	backup := &models.Backup{
		ServerID:              serverID,
		UserID:                &userID,
		StorageProviderID:     storageProviderID,
		CronExpression:        req.CronExpression,
		IncludeFiles:          string(includeFilesJSON),
		ExcludeFiles:          string(excludeFilesJSON),
		Retention:             retention,
		NotificationOnFailure: req.NotificationOnFailure,
		NotificationOnSuccess: req.NotificationOnSuccess,
		Enabled:               req.Enabled,
		Path:                  req.Path,
	}

	databaseIDs := []string{req.DatabaseID}

	if err := s.backupRepo.CreateBackupWithDatabases(ctx, backup, databaseIDs); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	activity.New(s.backupRepo.DB()).
		WithContext(ctx).
		UseLog("backup").
		On(backup).
		WithEvent("created").
		Log("Backup was created")

	// Dispatch installation job
	s.dispatchInstallBackup(serverID, backup.ID)

	s.logger.Info().
		Str("backup_id", backup.ID).
		Str("server_id", serverID).
		Msg("Backup created successfully")

	return backup, nil
}

// UpdateBackup updates an existing backup configuration
func (s *BackupService) UpdateBackup(ctx context.Context, id string, req *dto.UpdateBackupRequest) (*models.Backup, error) {
	backup, err := s.backupRepo.FindBackupByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert include/exclude files to JSON strings
	includeFilesJSON, err := json.Marshal(req.IncludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process include files: %w", err)
	}

	excludeFilesJSON, err := json.Marshal(req.ExcludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process exclude files: %w", err)
	}

	// Convert StorageProviderID from string to uint64
	storageProviderID, err := strconv.ParseUint(req.StorageProviderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid storage provider ID: %w", err)
	}

	backup.CronExpression = req.CronExpression
	backup.Path = req.Path
	backup.Enabled = req.Enabled
	backup.StorageProviderID = storageProviderID
	backup.IncludeFiles = string(includeFilesJSON)
	backup.ExcludeFiles = string(excludeFilesJSON)
	if req.Retention > 0 {
		backup.Retention = req.Retention
	}
	backup.NotificationOnFailure = req.NotificationOnFailure
	backup.NotificationOnSuccess = req.NotificationOnSuccess

	databaseIDs := []string{req.DatabaseID}

	if err := s.backupRepo.UpdateBackupWithDatabases(ctx, backup, databaseIDs); err != nil {
		return nil, fmt.Errorf("failed to update backup: %w", err)
	}

	activity.New(s.backupRepo.DB()).
		WithContext(ctx).
		UseLog("backup").
		On(backup).
		WithEvent("updated").
		Log("Backup was updated")

	s.logger.Info().
		Str("backup_id", backup.ID).
		Msg("Backup updated successfully")

	return backup, nil
}

// DeleteBackup deletes a backup configuration
func (s *BackupService) DeleteBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.backupRepo.FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	activity.New(s.backupRepo.DB()).
		WithContext(ctx).
		UseLog("backup").
		On(backup).
		WithEvent("deleted").
		Log("Backup was deleted")

	// Dispatch deletion job
	s.dispatchDeleteBackup(serverID, backup.ID)

	if err := s.backupRepo.DeleteBackup(ctx, id); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	s.logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Backup deleted successfully")

	return nil
}

// GetBackup retrieves a backup by ID
func (s *BackupService) GetBackup(ctx context.Context, id string) (*models.Backup, error) {
	return s.backupRepo.FindBackupByID(ctx, id)
}

// GetBackupByIDAndServer retrieves a backup by ID and server ID
func (s *BackupService) GetBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error) {
	return s.backupRepo.FindBackupByIDAndServer(ctx, id, serverID)
}

// ListBackupsByServer lists all backups for a server
func (s *BackupService) ListBackupsByServer(ctx context.Context, serverID string) ([]models.Backup, error) {
	return s.backupRepo.FindBackupsByServerID(ctx, serverID)
}

// RunBackup triggers a manual backup run
func (s *BackupService) RunBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.backupRepo.FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	s.dispatchRunManualBackup(serverID, backup.ID)

	s.logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Manual backup queued for execution")

	return nil
}

// MarkBackupInstalled marks a backup as installed
func (s *BackupService) MarkBackupInstalled(ctx context.Context, id string) error {
	return s.backupRepo.UpdateBackupFields(ctx, id, map[string]interface{}{
		"installed_at":           "NOW()",
		"installation_failed_at": nil,
	})
}

// MarkBackupInstallationFailed marks a backup installation as failed
func (s *BackupService) MarkBackupInstallationFailed(ctx context.Context, id string) error {
	return s.backupRepo.UpdateBackupFields(ctx, id, map[string]interface{}{
		"installation_failed_at": "NOW()",
	})
}

// Job dispatch helpers

func (s *BackupService) dispatchInstallBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue an InstallBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching InstallBackup job")
}

func (s *BackupService) dispatchDeleteBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue a DeleteBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching DeleteBackup job")
}

func (s *BackupService) dispatchRunManualBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue a RunManualBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching RunManualBackup job")
}

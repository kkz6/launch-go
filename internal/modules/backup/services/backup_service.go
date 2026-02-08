package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/jobs"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const defaultBackupRetention = 10

// BackupService handles business logic for backups
type BackupService struct {
	*BaseService
}

// NewBackupService creates a new backup service
func NewBackupService(deps *ServiceDeps) *BackupService {
	return &BackupService{
		BaseService: NewBaseService(deps),
	}
}

// CreateBackup creates a new backup configuration
func (s *BackupService) CreateBackup(ctx context.Context, serverID, userID, teamID string, req *dto.CreateBackupRequest) (*models.Backup, error) {
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
		retention = defaultBackupRetention
	}

	backup := &models.Backup{
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
	backup.ServerID = serverID
	backup.TeamID = teamID

	if err := s.Repos().Backup().CreateBackupWithDatabases(ctx, backup, req.Databases); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	activity.RecordEvent(ctx, "created", "", backup, "Backup was created")

	// Dispatch installation job
	s.dispatchInstallBackup(serverID, backup.ID)

	s.Logger.Info().
		Str("backup_id", backup.ID).
		Str("server_id", serverID).
		Msg("Backup created successfully")

	return backup, nil
}

// UpdateBackup updates an existing backup configuration
func (s *BackupService) UpdateBackup(ctx context.Context, id string, req *dto.UpdateBackupRequest) (*models.Backup, error) {
	backup, err := s.Repos().Backup().FindBackupByID(ctx, id)
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

	if err := s.Repos().Backup().UpdateBackupWithDatabases(ctx, backup, req.Databases); err != nil {
		return nil, fmt.Errorf("failed to update backup: %w", err)
	}

	activity.RecordEvent(ctx, "updated", "", backup, "Backup was updated")

	s.Logger.Info().
		Str("backup_id", backup.ID).
		Msg("Backup updated successfully")

	return backup, nil
}

// DeleteBackup deletes a backup configuration
func (s *BackupService) DeleteBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.Repos().Backup().FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", "", backup, "Backup was deleted")

	// Delete record first, then dispatch the cleanup job
	if err := s.Repos().Backup().DeleteBackup(ctx, id); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	// Dispatch deletion job to remove backup files from server.
	// If dispatch fails, the backup file remains on the server but the record is already gone.
	// This is the safer inconsistency since the record is the source of truth.
	s.dispatchDeleteBackup(serverID, backup.ID)

	s.Logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Backup deleted successfully")

	return nil
}

// GetBackup retrieves a backup by ID
func (s *BackupService) GetBackup(ctx context.Context, id string) (*models.Backup, error) {
	return s.Repos().Backup().FindBackupByID(ctx, id)
}

// GetBackupByIDAndServer retrieves a backup by ID and server ID
func (s *BackupService) GetBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error) {
	return s.Repos().Backup().FindBackupByIDAndServer(ctx, id, serverID)
}

// ListBackupsByServer lists all backups for a server
func (s *BackupService) ListBackupsByServer(ctx context.Context, serverID string) ([]models.Backup, error) {
	return s.Repos().Backup().FindBackupsByServerID(ctx, serverID)
}

// RunBackup triggers a manual backup run
func (s *BackupService) RunBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.Repos().Backup().FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	s.dispatchRunManualBackup(serverID, backup.ID)

	s.Logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Manual backup queued for execution")

	return nil
}

// MarkBackupInstalled marks a backup as installed
func (s *BackupService) MarkBackupInstalled(ctx context.Context, id string) error {
	return s.Repos().Backup().UpdateBackupFields(ctx, id, map[string]interface{}{
		"installed_at":           time.Now(),
		"installation_failed_at": nil,
	})
}

// MarkBackupInstallationFailed marks a backup installation as failed
func (s *BackupService) MarkBackupInstallationFailed(ctx context.Context, id string) error {
	return s.Repos().Backup().UpdateBackupFields(ctx, id, map[string]interface{}{
		"installation_failed_at": time.Now(),
	})
}

// Job dispatch helpers

func (s *BackupService) dispatchInstallBackup(serverID, backupID string) {
	s.DispatchTask("InstallBackup", func() (*asynq.Task, error) {
		return jobs.NewInstallBackupTask(serverID, backupID, nil)
	}, "server_id", serverID, "backup_id", backupID)
}

func (s *BackupService) dispatchDeleteBackup(serverID, backupID string) {
	s.DispatchTask("DeleteBackup", func() (*asynq.Task, error) {
		return jobs.NewDeleteBackupTask(serverID, backupID, nil)
	}, "server_id", serverID, "backup_id", backupID)
}

func (s *BackupService) dispatchRunManualBackup(serverID, backupID string) {
	s.DispatchTask("RunManualBackup", func() (*asynq.Task, error) {
		return jobs.NewRunManualBackupTask(serverID, backupID, nil)
	}, "server_id", serverID, "backup_id", backupID)
}

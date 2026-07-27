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

// BackupService handles business logic for backups. Service signatures
// follow the framework helper conventions:
//
//	IndexNested:        ListBackups(ctx, serverID, teamID)
//	ShowNested:         GetBackup(ctx, id, serverID, teamID)
//	CreateNested:       CreateBackup(ctx, serverID, teamID, userID, req)
//	UpdateNested:       UpdateBackup(ctx, id, serverID, teamID, userID, req)
//	DeleteNested:       DeleteBackup(ctx, id, serverID, teamID, userID)
//	ActionItemNested:   RunBackup(ctx, id, serverID, teamID, userID)
type BackupService struct {
	*BaseService
}

// NewBackupService creates a new backup service.
func NewBackupService(deps *ServiceDeps) *BackupService {
	return &BackupService{BaseService: NewBaseService(deps)}
}

// CreateBackup creates a new backup configuration and returns the response DTO.
func (s *BackupService) CreateBackup(ctx context.Context, serverID, teamID, userID string, req *dto.CreateBackupRequest) (dto.BackupResponse, error) {
	backup, err := s.buildAndDispatchBackup(ctx, serverID, teamID, userID, req)
	if err != nil {
		return dto.BackupResponse{}, err
	}
	return dto.ToBackupResponse(backup), nil
}

func (s *BackupService) buildAndDispatchBackup(ctx context.Context, serverID, teamID, userID string, req *dto.CreateBackupRequest) (*models.Backup, error) {
	includeFilesJSON, err := json.Marshal(req.IncludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process include files: %w", err)
	}
	excludeFilesJSON, err := json.Marshal(req.ExcludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process exclude files: %w", err)
	}
	storageProviderID, err := strconv.ParseUint(req.StorageProviderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid storage provider ID: %w", err)
	}

	retention := req.Retention
	if retention == 0 {
		retention = defaultBackupRetention
	}

	uid := userID
	backup := &models.Backup{
		UserID:                &uid,
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

	activity.RecordEvent(ctx, "created", userID, backup, "Backup was created")
	s.dispatchInstallBackup(serverID, backup.ID)

	s.Logger.Info().Str("backup_id", backup.ID).Str("server_id", serverID).Msg("Backup created successfully")
	return backup, nil
}

// UpdateBackup updates an existing backup configuration. The backup must
// belong to the given server.
func (s *BackupService) UpdateBackup(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateBackupRequest) (dto.BackupResponse, error) {
	backup, err := s.Repos().Backup().FindBackupByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return dto.BackupResponse{}, err
	}

	includeFilesJSON, err := json.Marshal(req.IncludeFiles)
	if err != nil {
		return dto.BackupResponse{}, fmt.Errorf("failed to process include files: %w", err)
	}
	excludeFilesJSON, err := json.Marshal(req.ExcludeFiles)
	if err != nil {
		return dto.BackupResponse{}, fmt.Errorf("failed to process exclude files: %w", err)
	}
	storageProviderID, err := strconv.ParseUint(req.StorageProviderID, 10, 64)
	if err != nil {
		return dto.BackupResponse{}, fmt.Errorf("invalid storage provider ID: %w", err)
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
		return dto.BackupResponse{}, fmt.Errorf("failed to update backup: %w", err)
	}

	activity.RecordEvent(ctx, "updated", userID, backup, "Backup was updated")
	s.Logger.Info().Str("backup_id", backup.ID).Msg("Backup updated successfully")

	return dto.ToBackupResponse(backup), nil
}

// DeleteBackup deletes a backup configuration.
func (s *BackupService) DeleteBackup(ctx context.Context, id, serverID, teamID, userID string) error {
	backup, err := s.Repos().Backup().FindBackupByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", userID, backup, "Backup was deleted")

	if err := s.Repos().Backup().DeleteBackup(ctx, id); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	// Dispatch deletion job to remove backup files from server. If
	// dispatch fails, the backup file remains on the server but the
	// record is already gone — the safer inconsistency.
	s.dispatchDeleteBackup(serverID, backup.ID)

	s.Logger.Info().Str("backup_id", id).Str("server_id", serverID).Msg("Backup deleted successfully")
	return nil
}

// GetBackup retrieves a backup by ID, verifying it belongs to the server.
// Signature matches ShowNestedFunc.
func (s *BackupService) GetBackup(ctx context.Context, id, serverID, teamID string) (dto.BackupResponse, error) {
	backup, err := s.Repos().Backup().FindBackupByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return dto.BackupResponse{}, err
	}
	return dto.ToBackupResponse(backup), nil
}

// ListBackups lists all backups for a server. Signature matches IndexNestedFunc.
func (s *BackupService) ListBackups(ctx context.Context, serverID, teamID string) ([]dto.BackupResponse, error) {
	backups, err := s.Repos().Backup().FindBackupsByServerAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BackupResponse, len(backups))
	for i := range backups {
		out[i] = dto.ToBackupResponse(&backups[i])
	}
	return out, nil
}

// RunBackup triggers a manual backup run. Signature matches ActionItemNestedFunc.
func (s *BackupService) RunBackup(ctx context.Context, id, serverID, teamID, userID string) error {
	_ = userID
	backup, err := s.Repos().Backup().FindBackupByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	s.dispatchRunManualBackup(serverID, backup.ID)
	s.Logger.Info().Str("backup_id", id).Str("server_id", serverID).Msg("Manual backup queued for execution")
	return nil
}

// GetBackupRaw is the model-returning entrypoint preserved for
// cross-service callers (job/agent-config services). HTTP handlers should
// use GetBackup (DTO-returning).
func (s *BackupService) GetBackupRaw(ctx context.Context, id string) (*models.Backup, error) {
	return s.Repos().Backup().FindBackupByID(ctx, id)
}

// MarkBackupInstalled marks a backup as installed.
func (s *BackupService) MarkBackupInstalled(ctx context.Context, id string) error {
	return s.Repos().Backup().UpdateBackupFields(ctx, id, map[string]interface{}{
		"installed_at":           time.Now(),
		"installation_failed_at": nil,
	})
}

// MarkBackupInstallationFailed marks a backup installation as failed.
func (s *BackupService) MarkBackupInstallationFailed(ctx context.Context, id string) error {
	return s.Repos().Backup().UpdateBackupFields(ctx, id, map[string]interface{}{
		"installation_failed_at": time.Now(),
	})
}

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

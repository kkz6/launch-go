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
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
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
//	Run action:         RunBackup(ctx, id, serverID, teamID, userID)
type BackupService struct {
	*BaseService
	dispatchManualBackup func(string, string, string, string, *string) error
}

// NewBackupService creates a new backup service.
func NewBackupService(deps *ServiceDeps) *BackupService {
	service := &BackupService{BaseService: NewBaseService(deps)}
	service.dispatchManualBackup = service.dispatchRunManualBackup
	return service
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
	if err := s.validateS3StorageProvider(ctx, storageProviderID, teamID); err != nil {
		return nil, err
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
	if err := s.validateS3StorageProvider(ctx, storageProviderID, teamID); err != nil {
		return dto.BackupResponse{}, err
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

// RunBackup creates and queues a manual backup run.
func (s *BackupService) RunBackup(
	ctx context.Context,
	id, serverID, teamID, userID string,
) (dto.BackupJobResponse, error) {
	backup, err := s.Repos().Backup().FindBackupForRun(ctx, id, serverID, teamID)
	if err != nil {
		return dto.BackupJobResponse{}, err
	}

	job := &models.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          backup.ID,
		StorageProviderID: backup.StorageProviderID,
	}
	job.TeamID = teamID
	if err := s.Repos().BackupJob().CreateBackupJob(ctx, job); err != nil {
		return dto.BackupJobResponse{}, fmt.Errorf("create manual backup run: %w", err)
	}

	if err := s.validateS3StorageProvider(ctx, backup.StorageProviderID, teamID); err != nil {
		message := "storage provider validation failed: " + err.Error()
		s.recordManualBackupFailure(ctx, job, backup, serverID, teamID, message)
		return dto.BackupJobResponse{}, fmt.Errorf("validate storage provider: %w", err)
	}

	if err := s.dispatchManualBackup(serverID, backup.ID, teamID, job.ID, &userID); err != nil {
		message := "failed to enqueue backup job: " + err.Error()
		s.recordManualBackupFailure(ctx, job, backup, serverID, teamID, message)
		return dto.BackupJobResponse{}, fmt.Errorf("queue manual backup: %w", err)
	}
	s.BroadcastToTeam(teamID, "backup.run.queued", map[string]any{
		"backup_id": backup.ID,
		"server_id": serverID,
		"team_id":   teamID,
		"job_id":    job.ID,
		"status":    string(backuptypes.BackupJobStatusPending),
	})
	s.Logger.Info().Str("backup_id", id).Str("server_id", serverID).Str("job_id", job.ID).
		Msg("Manual backup queued for execution")
	return dto.ToBackupJobResponse(job), nil
}

func (s *BackupService) recordManualBackupFailure(
	ctx context.Context,
	job *models.BackupJob,
	backup *models.Backup,
	serverID, teamID, message string,
) {
	persisted, updateErr := s.Repos().BackupJob().MarkBackupJobFailedForRun(
		ctx,
		job.ID,
		backup.ID,
		teamID,
		message,
		nil,
	)
	if updateErr != nil {
		s.LogError(updateErr, "failed to mark manual backup failure", "job_id", job.ID)
		return
	}
	if !persisted {
		return
	}

	job.Status = backuptypes.BackupJobStatusFailed
	job.Error = &message
	s.BroadcastToTeam(teamID, "backup.run.failed", map[string]any{
		"backup_id": backup.ID,
		"server_id": serverID,
		"team_id":   teamID,
		"job_id":    job.ID,
		"error":     message,
	})
}

func (s *BackupService) validateS3StorageProvider(ctx context.Context, providerID uint64, teamID string) error {
	driver, err := s.Repos().StorageProvider().FindStorageProviderDriverByIDAndTeam(ctx, providerID, teamID)
	if err != nil {
		return err
	}
	if driver != backuptypes.StorageDriverS3 {
		return ErrInvalidStorageDriver
	}
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

func (s *BackupService) dispatchRunManualBackup(
	serverID, backupID, teamID, jobID string,
	userID *string,
) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewRunManualBackupTask(serverID, backupID, teamID, jobID, userID)
	})
}

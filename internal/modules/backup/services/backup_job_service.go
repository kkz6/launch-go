package services

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/websocket"
)

// BackupJobService handles business logic for backup jobs
type BackupJobService struct {
	jobRepo    *repositories.BackupJobRepository
	backupRepo *repositories.BackupRepository
	ws         *websocket.Hub
	logger     *zerolog.Logger
}

// NewBackupJobService creates a new backup job service
func NewBackupJobService(
	jobRepo *repositories.BackupJobRepository,
	backupRepo *repositories.BackupRepository,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *BackupJobService {
	return &BackupJobService{
		jobRepo:    jobRepo,
		backupRepo: backupRepo,
		ws:         ws,
		logger:     logger,
	}
}

// CreateBackupJob creates a new backup job (called by agent webhook)
func (s *BackupJobService) CreateBackupJob(ctx context.Context, backupID, token string, req *dto.CreateBackupJobRequest) (*models.BackupJob, error) {
	backup, err := s.backupRepo.FindBackupByID(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Verify dispatch token
	if backup.DispatchToken != token {
		return nil, ErrInvalidDispatchToken
	}

	var errorMsg *string
	if req.Error != "" {
		errorMsg = &req.Error
	}

	job := &models.BackupJob{
		BackupID:          backupID,
		StorageProviderID: backup.StorageProviderID,
		Status:            req.Status,
		Size:              req.Size,
		Error:             errorMsg,
	}

	if err := s.jobRepo.CreateBackupJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create backup job: %w", err)
	}

	// Broadcast job status
	s.broadcastBackupJobStatus(backup.ServerID, job)

	s.logger.Info().
		Str("job_id", job.ID).
		Str("backup_id", backupID).
		Str("status", string(job.Status)).
		Msg("Backup job created")

	return job, nil
}

// GetBackupJob retrieves a backup job by ID
func (s *BackupJobService) GetBackupJob(ctx context.Context, id string) (*models.BackupJob, error) {
	return s.jobRepo.FindBackupJobByID(ctx, id)
}

// ListBackupJobs lists all jobs for a backup
func (s *BackupJobService) ListBackupJobs(ctx context.Context, backupID string) ([]models.BackupJob, error) {
	return s.jobRepo.FindBackupJobsByBackupID(ctx, backupID)
}

// Broadcast helpers

func (s *BackupJobService) broadcastBackupJobStatus(serverID string, job *models.BackupJob) {
	if s.ws == nil {
		return
	}

	s.ws.BroadcastToServer(serverID, "backup.job.status", map[string]interface{}{
		"job_id":    job.ID,
		"backup_id": job.BackupID,
		"status":    string(job.Status),
		"size":      job.Size,
	})
}

package services

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// BackupJobService handles business logic for backup jobs
type BackupJobService struct {
	*BaseService
}

// NewBackupJobService creates a new backup job service
func NewBackupJobService(deps *ServiceDeps) *BackupJobService {
	return &BackupJobService{
		BaseService: NewBaseService(deps),
	}
}

// CreateBackupJob creates a new backup job (called by agent webhook)
func (s *BackupJobService) CreateBackupJob(ctx context.Context, backupID, token string, req *dto.CreateBackupJobRequest) (*models.BackupJob, error) {
	backup, err := s.Repos().Backup().FindBackupByID(ctx, backupID)
	if err != nil {
		return nil, fiberutil.NotFoundAs(err, "Backup not found")
	}

	// Verify dispatch token using constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(backup.DispatchToken), []byte(token)) != 1 {
		return nil, ErrInvalidDispatchToken
	}

	var errorMsg *string
	if req.Error != "" {
		errorMsg = &req.Error
	}

	var size *int
	if req.Size > 0 {
		sizeInt := int(req.Size)
		size = &sizeInt
	}

	job := &models.BackupJob{
		BackupID:          backupID,
		StorageProviderID: backup.StorageProviderID,
		Status:            req.Status,
		Size:              size,
		Error:             errorMsg,
	}
	job.TeamID = backup.TeamID

	if err := s.Repos().BackupJob().CreateBackupJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create backup job: %w", err)
	}

	s.broadcastBackupJobStatus(backup.ServerID, backup.TeamID, job)

	s.Logger.Info().
		Str("job_id", job.ID).
		Str("backup_id", backupID).
		Str("status", string(job.Status)).
		Msg("Backup job created")

	return job, nil
}

// GetBackupJob retrieves a backup job by ID
func (s *BackupJobService) GetBackupJob(ctx context.Context, id string) (*models.BackupJob, error) {
	return s.Repos().BackupJob().FindBackupJobByID(ctx, id)
}

// ListBackupJobs lists all jobs for a backup
func (s *BackupJobService) ListBackupJobs(ctx context.Context, backupID string) ([]models.BackupJob, error) {
	return s.Repos().BackupJob().FindBackupJobsByBackupID(ctx, backupID)
}

// Broadcast helpers

func (s *BackupJobService) broadcastBackupJobStatus(serverID, teamID string, job *models.BackupJob) {
	payload := map[string]interface{}{
		"job_id":    job.ID,
		"backup_id": job.BackupID,
		"server_id": serverID,
		"team_id":   teamID,
		"status":    string(job.Status),
		"size":      job.Size,
		"error":     job.Error,
	}
	s.BroadcastToServer(serverID, "backup.job.status", payload)
	s.BroadcastToTeam(teamID, "backup.job.status", payload)
}
